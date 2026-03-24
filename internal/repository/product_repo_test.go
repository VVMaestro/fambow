package repository

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestProductRepositorySaveListRemove(t *testing.T) {
	ctx := context.Background()
	db := openProductTestDB(t)
	repo := NewProductRepository(db)

	product, err := repo.SaveProduct(ctx, "Flowers", 20)
	if err != nil {
		t.Fatalf("SaveProduct() unexpected error: %v", err)
	}
	if product.Name != "Flowers" || product.Cost != 20 {
		t.Fatalf("unexpected saved product: %#v", product)
	}

	products, err := repo.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts() unexpected error: %v", err)
	}
	if len(products) != 1 || products[0].ID != product.ID {
		t.Fatalf("unexpected listed products: %#v", products)
	}

	if err := repo.RemoveProduct(ctx, product.ID); err != nil {
		t.Fatalf("RemoveProduct() unexpected error: %v", err)
	}

	products, err = repo.ListProducts(ctx)
	if err != nil {
		t.Fatalf("ListProducts() after remove unexpected error: %v", err)
	}
	if len(products) != 0 {
		t.Fatalf("expected empty products after remove, got %#v", products)
	}
}

func TestProductRepositoryPurchaseProduct(t *testing.T) {
	t.Run("success deducts balance once", func(t *testing.T) {
		ctx := context.Background()
		db := openProductTestDB(t)
		repo := NewProductRepository(db)

		createProductTestUser(t, ctx, db, 101, "Anna", UserTypeWife, 100)
		createProductTestUser(t, ctx, db, 202, "Ivan", UserTypeHusband, 40)
		product := createProduct(t, ctx, db, "Flowers", 30)

		result, err := repo.PurchaseProduct(ctx, 101, product.ID)
		if err != nil {
			t.Fatalf("PurchaseProduct() unexpected error: %v", err)
		}
		if result.BuyerMoneyAfter != 70 {
			t.Fatalf("expected remaining money 70, got %d", result.BuyerMoneyAfter)
		}
		if result.OppositeTelegramUserID != 202 {
			t.Fatalf("expected opposite telegram id 202, got %d", result.OppositeTelegramUserID)
		}

		var money int64
		if err := db.QueryRowContext(ctx, `SELECT money FROM users WHERE telegram_user_id = ?`, 101).Scan(&money); err != nil {
			t.Fatalf("query buyer money: %v", err)
		}
		if money != 70 {
			t.Fatalf("expected buyer money 70 in DB, got %d", money)
		}
	})

	t.Run("fails with insufficient funds and keeps balance", func(t *testing.T) {
		ctx := context.Background()
		db := openProductTestDB(t)
		repo := NewProductRepository(db)

		createProductTestUser(t, ctx, db, 101, "Anna", UserTypeWife, 10)
		createProductTestUser(t, ctx, db, 202, "Ivan", UserTypeHusband, 0)
		product := createProduct(t, ctx, db, "Flowers", 30)

		_, err := repo.PurchaseProduct(ctx, 101, product.ID)
		if err != ErrProductInsufficientFunds {
			t.Fatalf("expected ErrProductInsufficientFunds, got %v", err)
		}

		var money int64
		if err := db.QueryRowContext(ctx, `SELECT money FROM users WHERE telegram_user_id = ?`, 101).Scan(&money); err != nil {
			t.Fatalf("query buyer money: %v", err)
		}
		if money != 10 {
			t.Fatalf("expected buyer money unchanged, got %d", money)
		}
	})

	t.Run("fails when opposite partner missing", func(t *testing.T) {
		ctx := context.Background()
		db := openProductTestDB(t)
		repo := NewProductRepository(db)

		createProductTestUser(t, ctx, db, 101, "Anna", UserTypeWife, 100)
		product := createProduct(t, ctx, db, "Flowers", 30)

		_, err := repo.PurchaseProduct(ctx, 101, product.ID)
		if err != ErrProductBuyerMissingPartner {
			t.Fatalf("expected ErrProductBuyerMissingPartner, got %v", err)
		}
	})

	t.Run("fails when buyer or product missing", func(t *testing.T) {
		ctx := context.Background()
		db := openProductTestDB(t)
		repo := NewProductRepository(db)

		createProductTestUser(t, ctx, db, 202, "Ivan", UserTypeHusband, 0)
		product := createProduct(t, ctx, db, "Flowers", 30)

		if _, err := repo.PurchaseProduct(ctx, 999, product.ID); err != ErrUserNotFound {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
		if _, err := repo.PurchaseProduct(ctx, 202, 999); err != ErrProductNotFound {
			t.Fatalf("expected ErrProductNotFound, got %v", err)
		}
	})
}

func TestUserRepositorySetMoneyByTelegramUserID(t *testing.T) {
	ctx := context.Background()
	db := openProductTestDB(t)
	repo := NewUserRepository(db)

	createProductTestUser(t, ctx, db, 101, "Anna", UserTypeWife, 10)

	user, err := repo.SetMoneyByTelegramUserID(ctx, 101, 75)
	if err != nil {
		t.Fatalf("SetMoneyByTelegramUserID() unexpected error: %v", err)
	}
	if user.Money != 75 {
		t.Fatalf("expected money 75, got %#v", user)
	}
}

func openProductTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := OpenSQLite(":memory:")
	if err != nil {
		t.Fatalf("OpenSQLite(): %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	migrationsDir := filepath.Join("..", "..", "migrations")
	if err := RunMigrations(context.Background(), db, migrationsDir); err != nil {
		t.Fatalf("RunMigrations(): %v", err)
	}

	return db
}

func createProductTestUser(t *testing.T, ctx context.Context, db *sql.DB, telegramUserID int64, firstName string, userType string, money int64) int64 {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO users (telegram_user_id, first_name, type, money)
		VALUES (?, ?, ?, ?)
	`, telegramUserID, firstName, userType, money)
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}

	userID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	return userID
}

func createProduct(t *testing.T, ctx context.Context, db *sql.DB, name string, cost int64) Product {
	t.Helper()

	result, err := db.ExecContext(ctx, `
		INSERT INTO products (name, cost)
		VALUES (?, ?)
	`, name, cost)
	if err != nil {
		t.Fatalf("insert test product: %v", err)
	}

	productID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId(): %v", err)
	}

	row := db.QueryRowContext(ctx, `
		SELECT id, name, cost, created_at
		FROM products
		WHERE id = ?
	`, productID)

	var product Product
	if err := row.Scan(&product.ID, &product.Name, &product.Cost, &product.CreatedAt); err != nil {
		t.Fatalf("scan test product: %v", err)
	}

	return product
}
