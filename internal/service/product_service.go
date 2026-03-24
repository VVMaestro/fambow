package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"fambow/internal/repository"
)

var ErrProductNameEmpty = errors.New("product name cannot be empty")
var ErrProductCostInvalid = errors.New("product cost is invalid")
var ErrProductIDInvalid = errors.New("product id is invalid")
var ErrProductNotFound = errors.New("product not found")
var ErrProductBuyerNotFound = errors.New("product buyer not found")
var ErrProductBuyerMissingPartner = errors.New("product buyer missing partner")
var ErrProductInsufficientFunds = errors.New("product insufficient funds")
var ErrMoneyAmountInvalid = errors.New("money amount is invalid")

type Product struct {
	ID   int64
	Name string
	Cost int64
}

type PurchaseResult struct {
	Product                Product
	BuyerTelegramUserID    int64
	BuyerFirstName         string
	BuyerType              string
	BuyerMoneyAfter        int64
	OppositeTelegramUserID int64
	OppositeFirstName      string
	OppositeType           string
}

type ProductStore interface {
	SaveProduct(ctx context.Context, name string, cost int64) (repository.Product, error)
	RemoveProduct(ctx context.Context, productID int64) error
	ListProducts(ctx context.Context) ([]repository.Product, error)
	PurchaseProduct(ctx context.Context, buyerTelegramUserID int64, productID int64) (repository.ProductPurchaseRecord, error)
}

type ProductService struct {
	store ProductStore
}

func NewProductService(store ProductStore) *ProductService {
	return &ProductService{store: store}
}

func (s *ProductService) AddProduct(ctx context.Context, command string) (Product, error) {
	name, cost, err := parseProductAddPayload(command)
	if err != nil {
		return Product{}, err
	}

	record, err := s.store.SaveProduct(ctx, name, cost)
	if err != nil {
		return Product{}, err
	}

	return mapProduct(record), nil
}

func (s *ProductService) RemoveProduct(ctx context.Context, command string) (int64, error) {
	productID, err := parseProductRemovePayload(command)
	if err != nil {
		return 0, err
	}

	if err := s.store.RemoveProduct(ctx, productID); err != nil {
		if errors.Is(err, repository.ErrProductNotFound) {
			return 0, ErrProductNotFound
		}
		return 0, err
	}

	return productID, nil
}

func (s *ProductService) ListProducts(ctx context.Context) ([]Product, error) {
	records, err := s.store.ListProducts(ctx)
	if err != nil {
		return nil, err
	}

	products := make([]Product, 0, len(records))
	for _, record := range records {
		products = append(products, mapProduct(record))
	}

	return products, nil
}

func (s *ProductService) BuyProduct(ctx context.Context, buyerTelegramUserID int64, productID int64) (PurchaseResult, error) {
	if buyerTelegramUserID <= 0 {
		return PurchaseResult{}, ErrUserTelegramIDInvalid
	}
	if productID <= 0 {
		return PurchaseResult{}, ErrProductIDInvalid
	}

	record, err := s.store.PurchaseProduct(ctx, buyerTelegramUserID, productID)
	if err != nil {
		switch {
		case errors.Is(err, repository.ErrProductNotFound):
			return PurchaseResult{}, ErrProductNotFound
		case errors.Is(err, repository.ErrUserNotFound):
			return PurchaseResult{}, ErrProductBuyerNotFound
		case errors.Is(err, repository.ErrProductBuyerMissingPartner):
			return PurchaseResult{}, ErrProductBuyerMissingPartner
		case errors.Is(err, repository.ErrProductInsufficientFunds):
			return PurchaseResult{}, ErrProductInsufficientFunds
		default:
			return PurchaseResult{}, err
		}
	}

	return PurchaseResult{
		Product: Product{
			ID:   record.ProductID,
			Name: record.ProductName,
			Cost: record.ProductCost,
		},
		BuyerTelegramUserID:    record.BuyerTelegramUserID,
		BuyerFirstName:         record.BuyerFirstName,
		BuyerType:              record.BuyerType,
		BuyerMoneyAfter:        record.BuyerMoneyAfter,
		OppositeTelegramUserID: record.OppositeTelegramUserID,
		OppositeFirstName:      record.OppositeFirstName,
		OppositeType:           record.OppositeType,
	}, nil
}

func parseProductAddPayload(command string) (string, int64, error) {
	parts := strings.Split(strings.TrimSpace(command), "|")
	if len(parts) != 2 {
		return "", 0, ErrProductCostInvalid
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return "", 0, ErrProductNameEmpty
	}

	cost, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || cost <= 0 {
		return "", 0, ErrProductCostInvalid
	}

	return name, cost, nil
}

func parseProductRemovePayload(command string) (int64, error) {
	productID, err := strconv.ParseInt(strings.TrimSpace(command), 10, 64)
	if err != nil || productID <= 0 {
		return 0, ErrProductIDInvalid
	}

	return productID, nil
}

func ParseMoneySetPayload(command string) (int64, int64, error) {
	parts := strings.Fields(strings.TrimSpace(command))
	if len(parts) != 2 {
		return 0, 0, ErrMoneyAmountInvalid
	}

	telegramUserID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || telegramUserID <= 0 {
		return 0, 0, ErrUserTelegramIDInvalid
	}

	money, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || money < 0 {
		return 0, 0, ErrMoneyAmountInvalid
	}

	return telegramUserID, money, nil
}

func ProductAddUsage() string {
	return "Use this format:\n/stuff_add <name> | <cost in pan-coins>"
}

func ProductRemoveUsage() string {
	return "Use this format:\n/stuff_remove <id>"
}

func MoneySetUsage() string {
	return "Use this format:\n/money_set <telegram_user_id> <amount in pan-coins>"
}

func FormatProduct(product Product) string {
	return fmt.Sprintf("#%d %s - %s", product.ID, product.Name, FormatPanCoins(product.Cost))
}

func FormatPanCoins(amount int64) string {
	if amount == 1 {
		return "1 pan-coin"
	}

	return fmt.Sprintf("%d pan-coins", amount)
}

func mapProduct(record repository.Product) Product {
	return Product{
		ID:   record.ID,
		Name: record.Name,
		Cost: record.Cost,
	}
}
