package entity

import "time"

type Order struct {
	Id                uint64     `gorm:"column:id;primaryKey;autoIncrement"`
	OrderId           string     `gorm:"column:order_id;unique;not null"`
	UserId            uint64     `gorm:"column:user_id;not null"`
	PackageId         uint64     `gorm:"column:package_id;not null"`
	GrossAmount       int64      `gorm:"column:gross_amount;not null"`
	PaymentType       string     `gorm:"column:payment_type;default:'qris'"`
	TransactionStatus string     `gorm:"column:transaction_status;default:'pending'"`
	QrString          string     `gorm:"column:qr_string"`
	QrUrl             string     `gorm:"column:qr_url"`
	MidtransResponse  string     `gorm:"column:midtrans_response;type:jsonb"`
	PaidAt            *time.Time `gorm:"column:paid_at"`
	CreatedAt         time.Time  `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;autoUpdateTime"`

	User    *User    `gorm:"foreignKey:UserId"`
	Package *Package `gorm:"foreignKey:PackageId"`
}

func (Order) TableName() string {
	return "orders"
}
