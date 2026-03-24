package service

import (
	"context"
	"errors"
	"testing"

	"fambow/internal/repository"
)

type productStoreSpy struct {
	saveName        string
	saveCost        int64
	saveResult      repository.Product
	saveErr         error
	removeProductID int64
	removeErr       error
	listResult      []repository.Product
	listErr         error
	buyBuyerID      int64
	buyProductID    int64
	buyResult       repository.ProductPurchaseRecord
	buyErr          error
}

func (s *productStoreSpy) SaveProduct(_ context.Context, name string, cost int64) (repository.Product, error) {
	s.saveName = name
	s.saveCost = cost
	return s.saveResult, s.saveErr
}

func (s *productStoreSpy) RemoveProduct(_ context.Context, productID int64) error {
	s.removeProductID = productID
	return s.removeErr
}

func (s *productStoreSpy) ListProducts(context.Context) ([]repository.Product, error) {
	return s.listResult, s.listErr
}

func (s *productStoreSpy) PurchaseProduct(_ context.Context, buyerTelegramUserID int64, productID int64) (repository.ProductPurchaseRecord, error) {
	s.buyBuyerID = buyerTelegramUserID
	s.buyProductID = productID
	return s.buyResult, s.buyErr
}

func TestProductServiceAddProductValidation(t *testing.T) {
	svc := NewProductService(&productStoreSpy{})

	tests := []struct {
		name    string
		command string
		wantErr error
	}{
		{name: "empty name", command: " | 10", wantErr: ErrProductNameEmpty},
		{name: "missing delimiter", command: "Flowers 10", wantErr: ErrProductCostInvalid},
		{name: "invalid cost", command: "Flowers | no", wantErr: ErrProductCostInvalid},
		{name: "non positive cost", command: "Flowers | 0", wantErr: ErrProductCostInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.AddProduct(context.Background(), tt.command)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProductServiceAddProductNormalizesInputs(t *testing.T) {
	store := &productStoreSpy{
		saveResult: repository.Product{ID: 3, Name: "Flowers", Cost: 20},
	}
	svc := NewProductService(store)

	product, err := svc.AddProduct(context.Background(), " Flowers  | 20 ")
	if err != nil {
		t.Fatalf("AddProduct() unexpected error: %v", err)
	}
	if store.saveName != "Flowers" || store.saveCost != 20 {
		t.Fatalf("unexpected normalized values: %q %d", store.saveName, store.saveCost)
	}
	if product.ID != 3 || product.Name != "Flowers" || product.Cost != 20 {
		t.Fatalf("unexpected returned product: %#v", product)
	}
}

func TestProductServiceRemoveProductValidation(t *testing.T) {
	svc := NewProductService(&productStoreSpy{})

	if _, err := svc.RemoveProduct(context.Background(), "nope"); !errors.Is(err, ErrProductIDInvalid) {
		t.Fatalf("expected ErrProductIDInvalid, got %v", err)
	}
}

func TestProductServiceRemoveProductMapsRepositoryErrors(t *testing.T) {
	store := &productStoreSpy{removeErr: repository.ErrProductNotFound}
	svc := NewProductService(store)

	if _, err := svc.RemoveProduct(context.Background(), "7"); !errors.Is(err, ErrProductNotFound) {
		t.Fatalf("expected ErrProductNotFound, got %v", err)
	}
}

func TestProductServiceBuyProductMapsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantErr error
	}{
		{name: "missing product", err: repository.ErrProductNotFound, wantErr: ErrProductNotFound},
		{name: "missing buyer", err: repository.ErrUserNotFound, wantErr: ErrProductBuyerNotFound},
		{name: "missing partner", err: repository.ErrProductBuyerMissingPartner, wantErr: ErrProductBuyerMissingPartner},
		{name: "insufficient funds", err: repository.ErrProductInsufficientFunds, wantErr: ErrProductInsufficientFunds},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &productStoreSpy{buyErr: tt.err}
			svc := NewProductService(store)

			_, err := svc.BuyProduct(context.Background(), 10, 20)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestProductServiceBuyProductSuccess(t *testing.T) {
	store := &productStoreSpy{
		buyResult: repository.ProductPurchaseRecord{
			ProductID:              8,
			ProductName:            "Flowers",
			ProductCost:            30,
			BuyerTelegramUserID:    101,
			BuyerFirstName:         "Anna",
			BuyerType:              "wife",
			BuyerMoneyAfter:        70,
			OppositeTelegramUserID: 202,
			OppositeFirstName:      "Ivan",
			OppositeType:           "husband",
		},
	}
	svc := NewProductService(store)

	result, err := svc.BuyProduct(context.Background(), 101, 8)
	if err != nil {
		t.Fatalf("BuyProduct() unexpected error: %v", err)
	}
	if store.buyBuyerID != 101 || store.buyProductID != 8 {
		t.Fatalf("unexpected buy inputs: %d %d", store.buyBuyerID, store.buyProductID)
	}
	if result.BuyerMoneyAfter != 70 || result.OppositeTelegramUserID != 202 || result.Product.Name != "Flowers" {
		t.Fatalf("unexpected purchase result: %#v", result)
	}
}

func TestParseMoneySetPayload(t *testing.T) {
	telegramUserID, money, err := ParseMoneySetPayload("123 50")
	if err != nil {
		t.Fatalf("parseMoneySetPayload() unexpected error: %v", err)
	}
	if telegramUserID != 123 || money != 50 {
		t.Fatalf("unexpected parsed values: %d %d", telegramUserID, money)
	}

	if _, _, err := ParseMoneySetPayload("123 -1"); !errors.Is(err, ErrMoneyAmountInvalid) {
		t.Fatalf("expected ErrMoneyAmountInvalid, got %v", err)
	}
}
