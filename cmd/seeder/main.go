package main

import (
	"go-rich-buddy-platform/cmd/injector"
	"go-rich-buddy-platform/seeders"
	"log"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

func main() {
	viperConfig := injector.NewViperConfig()
	databaseCredential := injector.NewDatabaseCredentials(viperConfig)
	gormInstance := injector.NewDatabaseConnection(databaseCredential)
	viper.AutomaticEnv()

	// Read SEEDER env
	seederEnv := viper.GetString("SEEDER")

	if seederEnv == "" {
		log.Fatal("Please set SEEDER environment variable, e.g. SEEDER=all or SEEDER=UserSeeder")
	}

	seederNames := []string{}
	if strings.ToLower(seederEnv) == "all" {
		for name := range seeders.SeederRegistry {
			seederNames = append(seederNames, name)
		}
	} else {
		seederNames = append(seederNames, seederEnv)
	}

	for _, seederName := range seederNames {
		seederInstance, err := seeders.GetSeeder(seederName)
		if err != nil {
			log.Println(err)
			continue
		}
		logrus.Debug("Running seeder:", seederName)
		if err := seederInstance.Run(gormInstance); err != nil {
			log.Println("Seeder failed:", err)
		} else {
			logrus.Debug("Seeder completed:", seederName)
		}
	}
}

func CloseDB(gormDatabase *gorm.DB) {
	sqlDatabase, err := gormDatabase.DB()
	if err != nil {
		return
	}
	sqlDatabase.Close()
}
