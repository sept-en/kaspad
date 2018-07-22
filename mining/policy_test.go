// Copyright (c) 2016 The btcsuite developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package mining

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/daglabs/btcd/blockdag"
	"github.com/daglabs/btcd/btcec"
	"github.com/daglabs/btcd/dagconfig"
	"github.com/daglabs/btcd/dagconfig/daghash"
	"github.com/daglabs/btcd/txscript"
	"github.com/daglabs/btcd/wire"
	"github.com/daglabs/btcutil"
)

// newHashFromStr converts the passed big-endian hex string into a
// daghash.Hash.  It only differs from the one available in daghash in that
// it panics on an error since it will only (and must only) be called with
// hard-coded, and therefore known good, hashes.
func newHashFromStr(hexStr string) *daghash.Hash {
	hash, err := daghash.NewHashFromStr(hexStr)
	if err != nil {
		panic("invalid hash in source file: " + hexStr)
	}
	return hash
}

// hexToBytes converts the passed hex string into bytes and will panic if there
// is an error.  This is only provided for the hard-coded constants so errors in
// the source code can be detected. It will only (and must only) be called with
// hard-coded values.
func hexToBytes(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("invalid hex in source file: " + s)
	}
	return b
}

// newUtxoViewpoint returns a new utxo view populated with outputs of the
// provided source transactions as if there were available at the respective
// block height specified in the heights slice.  The length of the source txns
// and source tx heights must match or it will panic.
func newUtxoViewpoint(sourceTxns []*wire.MsgTx, sourceTxHeights []int32) *blockdag.UtxoViewpoint {
	if len(sourceTxns) != len(sourceTxHeights) {
		panic("each transaction must have its block height specified")
	}

	view := blockdag.NewUtxoViewpoint()
	for i, tx := range sourceTxns {
		view.AddTxOuts(btcutil.NewTx(tx), sourceTxHeights[i])
	}
	return view
}

func getTxIn(originTx *wire.MsgTx, outputIndex uint32) *wire.TxIn {
	var prevOut *wire.OutPoint
	if originTx != nil {
		originTxHash := originTx.TxHash()
		prevOut = wire.NewOutPoint(&originTxHash, 0)
	} else {
		prevOut = &wire.OutPoint{
			Hash:  daghash.Hash{},
			Index: 0xFFFFFFFF,
		}
	}
	return wire.NewTxIn(prevOut, nil)
}

func createTransaction(value int64, originTx *wire.MsgTx, outputIndex uint32, sigScript []byte) (*wire.MsgTx, error) {
	lookupKey := func(a btcutil.Address) (*btcec.PrivateKey, bool, error) {
		// Ordinarily this function would involve looking up the private
		// key for the provided address, but since the only thing being
		// signed in this example uses the address associated with the
		// private key from above, simply return it with the compressed
		// flag set since the address is using the associated compressed
		// public key.
		//
		// NOTE: If you want to prove the code is actually signing the
		// transaction properly, uncomment the following line which
		// intentionally returns an invalid key to sign with, which in
		// turn will result in a failure during the script execution
		// when verifying the signature.
		//
		// privKey.D.SetInt64(12345)
		//
		return privKey, true, nil
	}

	redeemTx := wire.NewMsgTx(wire.TxVersion)

	redeemTx.AddTxIn(getTxIn(originTx, outputIndex))

	pkScript, err := txscript.PayToAddrScript(addr)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	txOut := wire.NewTxOut(value, pkScript)
	redeemTx.AddTxOut(txOut)
	if sigScript == nil {
		sigScript, err = txscript.SignTxOutput(&dagconfig.MainNetParams,
			redeemTx, 0, originTx.TxOut[0].PkScript, txscript.SigHashAll,
			txscript.KeyClosure(lookupKey), nil, nil)
	}
	redeemTx.TxIn[0].SignatureScript = sigScript
	return redeemTx, nil
}

// TestCalcPriority ensures the priority calculations work as intended.
func TestCalcPriority(t *testing.T) {
	// commonSourceTx1 is a valid transaction used in the tests below as an
	// input to transactions that are having their priority calculated.
	//
	// From block 7 in main blockchain.
	// tx 0437cd7f8525ceed2324359c2d0ba26006d92d856a9c20fa0241106ee5a597c9
	commonSourceTx1, err := createTransaction(5000000000, nil, 0, hexToBytes("04ffff001d0134"))

	if err != nil {
		t.Errorf("Error with creating source tx: %v", err)
	}

	// commonRedeemTx1 is a valid transaction used in the tests below as the
	// transaction to calculate the priority for.
	//
	// It originally came from block 170 in main blockchain.
	commonRedeemTx1, err := createTransaction(5000000000, commonSourceTx1, 0, nil)

	if err != nil {
		t.Errorf("Error with creating redeem tx: %v", err)
	}

	tests := []struct {
		name       string                  // test description
		tx         *wire.MsgTx             // tx to calc priority for
		utxoView   *blockdag.UtxoViewpoint // inputs to tx
		nextHeight int32                   // height for priority calc
		want       float64                 // expected priority
	}{
		{
			name: "one height 7 input, prio tx height 169",
			tx:   commonRedeemTx1,
			utxoView: newUtxoViewpoint([]*wire.MsgTx{commonSourceTx1},
				[]int32{7}),
			nextHeight: 169,
			want:       1.5576923076923077e+10,
		},
		{
			name: "one height 100 input, prio tx height 169",
			tx:   commonRedeemTx1,
			utxoView: newUtxoViewpoint([]*wire.MsgTx{commonSourceTx1},
				[]int32{100}),
			nextHeight: 169,
			want:       6.634615384615385e+09,
		},
		{
			name: "one height 7 input, prio tx height 100000",
			tx:   commonRedeemTx1,
			utxoView: newUtxoViewpoint([]*wire.MsgTx{commonSourceTx1},
				[]int32{7}),
			nextHeight: 100000,
			want:       9.61471153846154e+12,
		},
		{
			name: "one height 100 input, prio tx height 100000",
			tx:   commonRedeemTx1,
			utxoView: newUtxoViewpoint([]*wire.MsgTx{commonSourceTx1},
				[]int32{100}),
			nextHeight: 100000,
			want:       9.60576923076923e+12,
		},
	}

	for i, test := range tests {
		got := CalcPriority(test.tx, test.utxoView, test.nextHeight)
		if got != test.want {
			t.Errorf("CalcPriority #%d (%q): unexpected priority "+
				"got %v want %v", i, test.name, got, test.want)
			continue
		}
	}
}

var privKeyBytes, _ = hex.DecodeString("22a47fa09a223f2aa079edf85a7c2" +
	"d4f8720ee63e502ee2869afab7de234b80c")

var privKey, pubKey = btcec.PrivKeyFromBytes(btcec.S256(), privKeyBytes)
var pubKeyHash = btcutil.Hash160(pubKey.SerializeCompressed())
var addr, err = btcutil.NewAddressPubKeyHash(pubKeyHash,
	&dagconfig.MainNetParams)
