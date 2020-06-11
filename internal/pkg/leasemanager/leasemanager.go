package leasemanager

type Subject interface {
	Register(Observer)
	Deregister(Observer)
	NotifyAllNewLease()
	NotifyAllStaleLease()
}

type Observer interface {
	NotifyNewLease()
	NotifyStaleLease()
	GetID() string
}

func RemoveObserver(observers []Observer, observerToRemove Observer) []Observer {
	observersLen := len(observers)
	for idx, observer := range observers {
		if observerToRemove.GetID() == observer.GetID() {
			observers[observersLen-1], observers[idx] = observers[idx], observers[observersLen-1]
			return observers[:observersLen-1]
		}
	}
	return observers
}
