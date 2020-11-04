package rpccontext

import (
	"sync"

	"github.com/kaspanet/kaspad/app/appmessage"
	routerpkg "github.com/kaspanet/kaspad/infrastructure/network/netadapter/router"
	"github.com/kaspanet/kaspad/util"
	"github.com/kaspanet/kaspad/util/daghash"
	"github.com/pkg/errors"
)

// NotificationManager manages notifications for the RPC
type NotificationManager struct {
	sync.RWMutex
	listeners map[*routerpkg.Router]*NotificationListener
}

// NotificationListener represents a registered RPC notification listener
type NotificationListener struct {
	propagateBlockAddedNotifications               bool
	propagateTransactionAddedNotifications         bool
	propagateChainChangedNotifications             bool
	propagateFinalityConflictNotifications         bool
	propagateFinalityConflictResolvedNotifications bool
	propagateUTXOOfAddressChangedNotifications     bool
	subscribedTransactions                         map[daghash.Hash]struct{}
	subscribedAddresses                            map[string]struct{}
}

// NewNotificationManager creates a new NotificationManager
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		listeners: make(map[*routerpkg.Router]*NotificationListener),
	}
}

// AddListener registers a listener with the given router
func (nm *NotificationManager) AddListener(router *routerpkg.Router) {
	nm.Lock()
	defer nm.Unlock()

	listener := newNotificationListener()
	nm.listeners[router] = listener
}

// RemoveListener unregisters the given router
func (nm *NotificationManager) RemoveListener(router *routerpkg.Router) {
	nm.Lock()
	defer nm.Unlock()

	delete(nm.listeners, router)
}

// Listener retrieves the listener registered with the given router
func (nm *NotificationManager) Listener(router *routerpkg.Router) (*NotificationListener, error) {
	nm.RLock()
	defer nm.RUnlock()

	listener, ok := nm.listeners[router]
	if !ok {
		return nil, errors.Errorf("listener not found")
	}
	return listener, nil
}

// NotifyBlockAdded notifies the notification manager that a block has been added to the DAG
func (nm *NotificationManager) NotifyBlockAdded(notification *appmessage.BlockAddedNotificationMessage) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateBlockAddedNotifications {
			err := router.OutgoingRoute().Enqueue(notification)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// NotifyTransactionAdded notifies the notification manager that a transaction has been added to the DAG
func (nm *NotificationManager) NotifyTransactionAdded(transactions []*util.Tx) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateTransactionAddedNotifications {
			for _, tx := range transactions {
				if _, ok := listener.subscribedTransactions[*tx.Hash()]; ok {
					delete(listener.subscribedTransactions, *tx.Hash())
					notification := appmessage.NewTransactionAddedNotificationMessage(tx.MsgTx())
					err := router.OutgoingRoute().Enqueue(notification)
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// NotifyUTXOOfAddressChanged notifies the notification manager that a ssociated utxo set with address was changed
func (nm *NotificationManager) NotifyUTXOOfAddressChanged(notification *appmessage.UTXOOfAddressChangedNotificationMessage) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateUTXOOfAddressChangedNotifications {
			changedAddressesForListener := []string{}
			for _, address := range notification.ChangedAddresses {
				if _, ok := listener.subscribedAddresses[address]; ok {
					changedAddressesForListener = append(changedAddressesForListener, address)
				}
			}

			if len(changedAddressesForListener) > 0 {
				notification := appmessage.NewUTXOOfAddressChangedNotificationMessage(changedAddressesForListener)
				err := router.OutgoingRoute().Enqueue(notification)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// NotifyChainChanged notifies the notification manager that the DAG's selected parent chain has changed
func (nm *NotificationManager) NotifyChainChanged(notification *appmessage.ChainChangedNotificationMessage) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateChainChangedNotifications {
			err := router.OutgoingRoute().Enqueue(notification)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// NotifyFinalityConflict notifies the notification manager that there's a finality conflict in the DAG
func (nm *NotificationManager) NotifyFinalityConflict(notification *appmessage.FinalityConflictNotificationMessage) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateFinalityConflictNotifications {
			err := router.OutgoingRoute().Enqueue(notification)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// NotifyFinalityConflictResolved notifies the notification manager that a finality conflict in the DAG has been resolved
func (nm *NotificationManager) NotifyFinalityConflictResolved(notification *appmessage.FinalityConflictResolvedNotificationMessage) error {
	nm.RLock()
	defer nm.RUnlock()

	for router, listener := range nm.listeners {
		if listener.propagateFinalityConflictResolvedNotifications {
			err := router.OutgoingRoute().Enqueue(notification)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func newNotificationListener() *NotificationListener {
	return &NotificationListener{
		propagateBlockAddedNotifications:               false,
		propagateTransactionAddedNotifications:         false,
		propagateChainChangedNotifications:             false,
		propagateFinalityConflictNotifications:         false,
		propagateFinalityConflictResolvedNotifications: false,
	}
}

// PropagateBlockAddedNotifications instructs the listener to send block added notifications
// to the remote listener
func (nl *NotificationListener) PropagateBlockAddedNotifications() {
	nl.propagateBlockAddedNotifications = true
}

// PropagateTransactionAddedNotifications instructs the listener to send transaction added notifications
// to the remote listener
func (nl *NotificationListener) PropagateTransactionAddedNotifications(txHash *daghash.Hash) {
	nl.propagateTransactionAddedNotifications = true

	if nl.subscribedTransactions == nil {
		nl.subscribedTransactions = make(map[daghash.Hash]struct{})
	}

	nl.subscribedTransactions[*txHash] = struct{}{}
}

// PropagateUTXOOfAddressChangedNotifications instructs the listener to send utxo of address changed notifications
// to the remote listener
func (nl *NotificationListener) PropagateUTXOOfAddressChangedNotifications(addresses []string) {
	nl.propagateUTXOOfAddressChangedNotifications = true

	if nl.subscribedAddresses == nil {
		nl.subscribedAddresses = make(map[string]struct{})
	}

	for _, address := range addresses {
		nl.subscribedAddresses[address] = struct{}{}
	}
}

// PropagateChainChangedNotifications instructs the listener to send chain changed notifications
// to the remote listener
func (nl *NotificationListener) PropagateChainChangedNotifications() {
	nl.propagateChainChangedNotifications = true
}

// PropagateFinalityConflictNotifications instructs the listener to send finality conflict notifications
// to the remote listener
func (nl *NotificationListener) PropagateFinalityConflictNotifications() {
	nl.propagateFinalityConflictNotifications = true
}

// PropagateFinalityConflictResolvedNotifications instructs the listener to send finality conflict resolved notifications
// to the remote listener
func (nl *NotificationListener) PropagateFinalityConflictResolvedNotifications() {
	nl.propagateFinalityConflictResolvedNotifications = true
}
