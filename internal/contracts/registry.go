package contracts

// EventSnapshotChanged carries a SnapshotChanged
const EventSnapshotChanged MessageType = "snapshot_changed"

// SnapshotChanged announces a change of the runtime read model (it carries nothing; a subscriber reads the registry)
type SnapshotChanged struct{}
