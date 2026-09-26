package config

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type DatabaseCredentials struct {
	DatabaseUsername string
	DatabasePassword string
	DatabaseHost     string
	DatabasePort     string
	DatabaseName     string
}
type DatabaseConnection struct {
	databaseInstance    *gorm.DB
	databaseCredentials *DatabaseCredentials
}

func NewDatabaseConnection(databaseCredentials *DatabaseCredentials) *DatabaseConnection {
	return &DatabaseConnection{
		databaseCredentials: databaseCredentials,
	}
}

func (dbConn *DatabaseConnection) GetDatabaseConnection() *gorm.DB {

	if dbConn.databaseInstance == nil {
		sqlDialect := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable TimeZone=Asia/Jakarta",
			dbConn.databaseCredentials.DatabaseUsername,
			dbConn.databaseCredentials.DatabasePassword,
			dbConn.databaseCredentials.DatabaseHost,
			dbConn.databaseCredentials.DatabasePort,
			dbConn.databaseCredentials.DatabaseName)

		databaseConnection, err := gorm.Open(postgres.Open(sqlDialect), &gorm.Config{})
		if err != nil {
			panic(err)
		}
		dbConn.databaseInstance = databaseConnection
	}
	return dbConn.databaseInstance
}
