package entity

type User struct {
	Id            uint64 `gorm:"column:id;primaryKey;autoIncrement"`
	Name          string `gorm:"column:name"`
	Phone         string `gorm:"column:phone"`
	Email         string `gorm:"column:email"`
	Password      string `gorm:"column:password"`
	Tier          string `gorm:"column:tier;default:'Starter'"`
	CreditBalance int    `gorm:"column:credit_balance;default:5"`
}
