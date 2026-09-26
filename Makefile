.PHONY: migrate-fresh
.PHONY: migrate-up
.PHONY: migrate-down

migrate-fresh:
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" down
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" up

migrate-up:
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" up 1

migrate:
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" up

migrate-down:
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" down 1

migrate-force:
	migrate -path migrations -database "postgres://postgres:@127.0.0.1:5432/go_rich_buddy_platform?sslmode=disable" force $(version)

migrate-create:
	migrate create -ext sql -dir migrations/ create_$(name)_table

migrate-alter:
	migrate create -ext sql -dir migrations/ alter_$(name)_table
