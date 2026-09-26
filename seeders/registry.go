package seeders

import (
	"errors"
)

// Map nama seeder → struct
var SeederRegistry = map[string]Seeder{}

// Ambil seeder dari nama
func GetSeeder(name string) (Seeder, error) {
	if seeder, ok := SeederRegistry[name]; ok {
		return seeder, nil
	}
	return nil, errors.New("Seeder not found: " + name)
}
