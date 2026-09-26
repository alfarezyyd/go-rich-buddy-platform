package seeders

import (
	"errors"
)

var SeederRegistry = map[string]Seeder{}

func GetSeeder(name string) (Seeder, error) {
	if seeder, ok := SeederRegistry[name]; ok {
		return seeder, nil
	}
	return nil, errors.New("Seeder not found: " + name)
}
