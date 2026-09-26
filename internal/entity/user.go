package entity

type User struct {
	Id        uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	Name      string    `gorm:"column:name"`
	Phone     string    `gorm:"column:phone"`
	Email     string    `gorm:"column:email"`
	Password  string    `gorm:"column:password"`
	Auditable Auditable `gorm:"embedded"`
}

func (userEntity *User) GetAuditable() *Auditable {
	return &userEntity.Auditable
}
