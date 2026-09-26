package trait

type HasId interface {
	GetId() uint64
}

type HasUniqueId interface {
	GetUniqueId() string
}
