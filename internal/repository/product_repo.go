package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrProductNotFound = errors.New("product not found")
var ErrProductBuyerMissingPartner = errors.New("product buyer missing partner")
var ErrProductInsufficientFunds = errors.New("product insufficient funds")

type ProductPurchaseRecord struct {
	ProductID              int64
	ProductName            string
	ProductCost            int64
	BuyerTelegramUserID    int64
	BuyerFirstName         string
	BuyerType              string
	BuyerMoneyAfter        int64
	OppositeTelegramUserID int64
	OppositeFirstName      string
	OppositeType           string
}

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) SaveProduct(ctx context.Context, name string, cost int64) (Product, error) {
	result, err := r.db.ExecContext(ctx, `
		INSERT INTO products (name, cost)
		VALUES (?, ?)
	`, name, cost)
	if err != nil {
		return Product{}, fmt.Errorf("insert product: %w", err)
	}

	productID, err := result.LastInsertId()
	if err != nil {
		return Product{}, fmt.Errorf("get inserted product id: %w", err)
	}

	return r.findProductByID(ctx, productID)
}

func (r *ProductRepository) RemoveProduct(ctx context.Context, productID int64) error {
	result, err := r.db.ExecContext(ctx, `
		DELETE FROM products
		WHERE id = ?
	`, productID)
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deleted product rows: %w", err)
	}
	if rowsAffected == 0 {
		return ErrProductNotFound
	}

	return nil
}

func (r *ProductRepository) ListProducts(ctx context.Context) ([]Product, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, cost, created_at
		FROM products
		ORDER BY lower(name) ASC, id ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		product, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, product)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate listed products: %w", err)
	}

	return products, nil
}

func (r *ProductRepository) PurchaseProduct(ctx context.Context, buyerTelegramUserID int64, productID int64) (ProductPurchaseRecord, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ProductPurchaseRecord{}, fmt.Errorf("begin product purchase transaction: %w", err)
	}
	defer tx.Rollback()

	product, err := findProductByIDTx(ctx, tx, productID)
	if err != nil {
		return ProductPurchaseRecord{}, err
	}

	buyer, err := findUserByTelegramUserIDTx(ctx, tx, buyerTelegramUserID)
	if err != nil {
		return ProductPurchaseRecord{}, err
	}

	if buyer.Money < product.Cost {
		return ProductPurchaseRecord{}, ErrProductInsufficientFunds
	}

	oppositeType := oppositeUserType(buyer.Type)
	if oppositeType == "" {
		return ProductPurchaseRecord{}, ErrProductBuyerMissingPartner
	}

	opposite, err := findUserByTypeTx(ctx, tx, oppositeType)
	if err != nil {
		if errors.Is(err, ErrUserTypeNotFound) {
			return ProductPurchaseRecord{}, ErrProductBuyerMissingPartner
		}
		return ProductPurchaseRecord{}, err
	}

	newMoney := buyer.Money - product.Cost
	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET money = ?
		WHERE id = ?
	`, newMoney, buyer.ID); err != nil {
		return ProductPurchaseRecord{}, fmt.Errorf("deduct buyer money: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ProductPurchaseRecord{}, fmt.Errorf("commit product purchase transaction: %w", err)
	}

	return ProductPurchaseRecord{
		ProductID:              product.ID,
		ProductName:            product.Name,
		ProductCost:            product.Cost,
		BuyerTelegramUserID:    buyer.TelegramUserID,
		BuyerFirstName:         buyer.FirstName,
		BuyerType:              buyer.Type,
		BuyerMoneyAfter:        newMoney,
		OppositeTelegramUserID: opposite.TelegramUserID,
		OppositeFirstName:      opposite.FirstName,
		OppositeType:           opposite.Type,
	}, nil
}

func (r *ProductRepository) findProductByID(ctx context.Context, productID int64) (Product, error) {
	return findProductByIDTx(ctx, r.db, productID)
}

func findProductByIDTx(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, productID int64) (Product, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, cost, created_at
		FROM products
		WHERE id = ?
		LIMIT 1
	`, productID)

	var product Product
	if err := row.Scan(&product.ID, &product.Name, &product.Cost, &product.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Product{}, ErrProductNotFound
		}
		return Product{}, fmt.Errorf("find product by id: %w", err)
	}

	return product, nil
}

func scanProduct(scanner interface {
	Scan(dest ...any) error
}) (Product, error) {
	var product Product
	if err := scanner.Scan(&product.ID, &product.Name, &product.Cost, &product.CreatedAt); err != nil {
		return Product{}, fmt.Errorf("scan product: %w", err)
	}

	return product, nil
}

func findUserByTelegramUserIDTx(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, telegramUserID int64) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, first_name, type, money, created_at
		FROM users
		WHERE telegram_user_id = ?
		LIMIT 1
	`, telegramUserID)

	var user User
	if err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.Money, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, fmt.Errorf("find user by telegram id: %w", err)
	}

	return user, nil
}

func findUserByTypeTx(ctx context.Context, db interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}, userType string) (User, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, telegram_user_id, first_name, type, money, created_at
		FROM users
		WHERE lower(type) = lower(?)
		LIMIT 1
	`, userType)

	var user User
	if err := row.Scan(&user.ID, &user.TelegramUserID, &user.FirstName, &user.Type, &user.Money, &user.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserTypeNotFound
		}
		return User{}, fmt.Errorf("find user by type: %w", err)
	}

	return user, nil
}

func oppositeUserType(userType string) string {
	switch userType {
	case UserTypeHusband:
		return UserTypeWife
	case UserTypeWife:
		return UserTypeHusband
	default:
		return ""
	}
}
