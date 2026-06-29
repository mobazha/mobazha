//go:build !private_distribution

package core

import (
	"context"
	"errors"
	"testing"
	"time"

	coreorder "github.com/mobazha/mobazha3.0/internal/core/order"
	"github.com/mobazha/mobazha3.0/internal/repo"
	"github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/models"
	pb "github.com/mobazha/mobazha3.0/pkg/orders/mbzpb"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestCollectibleLifecycleDeliveryRetriesPaidHookUntilAcknowledged(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	orderSvc := coreorder.NewTestOrderAppService(t, coreorder.OrderAppServiceConfig{DB: db})
	orderID := "collectible-delivery-retry"
	coreorder.SeedOrder(t, orderSvc, orderID, string(models.RoleVendor), models.OrderState_AWAITING_FULFILLMENT)

	paymentSent := &pb.PaymentSent{
		TransactionID: "tx-retry",
		Coin:          "crypto:eip155:1:native",
		Amount:        "100",
		ToAddress:     "0x111122223333444455556666777788889999aaaa",
	}
	serializedPayment, err := protojson.Marshal(paymentSent)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.CollectibleLifecycleDelivery{}); err != nil {
			return err
		}
		var order models.Order
		if err := tx.Read().Where("id = ?", orderID).First(&order).Error; err != nil {
			return err
		}
		order.SerializedPaymentSent = serializedPayment
		serializedOrderOpen, err := protojson.Marshal(collectibleLifecycleOrderOpen(
			"11111111111111111111111111111111",
			nil,
		))
		if err != nil {
			return err
		}
		order.SerializedOrderOpen = serializedOrderOpen
		if err := tx.Save(&order); err != nil {
			return err
		}
		return tx.Save(&models.CollectibleLifecycleDelivery{
			JobID:   models.CollectibleLifecyclePaid + ":" + orderID,
			OrderID: orderID,
			Kind:    models.CollectibleLifecyclePaid,
		})
	}); err != nil {
		t.Fatal(err)
	}

	calls := 0
	node := &MobazhaNode{
		storageFields: storageFields{db: db},
		appServices:   appServices{orderService: orderSvc},
		collectiblesFields: collectiblesFields{
			collectiblePrimarySalePaidHook: func(context.Context, CollectiblePrimarySalePaidSignal) error {
				calls++
				if calls == 1 {
					return errors.New("hosting temporarily unavailable")
				}
				return nil
			},
		},
	}

	node.runCollectibleLifecycleDeliveries(context.Background())
	var job models.CollectibleLifecycleDelivery
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("job_id = ?", models.CollectibleLifecyclePaid+":"+orderID).First(&job).Error
	}); err != nil {
		t.Fatal(err)
	}
	if job.Attempts != 1 || job.DeliveredAt != nil || job.NextAttemptAt == nil {
		t.Fatalf("failed delivery was not retained for retry: %+v", job)
	}

	past := time.Now().UTC().Add(-time.Second)
	if err := db.Update(func(tx database.Tx) error {
		return tx.Update("next_attempt_at", past, map[string]interface{}{"job_id = ?": job.JobID}, &models.CollectibleLifecycleDelivery{})
	}); err != nil {
		t.Fatal(err)
	}
	node.runCollectibleLifecycleDeliveries(context.Background())
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("job_id = ?", job.JobID).First(&job).Error
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || job.Attempts != 2 || job.DeliveredAt == nil || job.LastError != "" {
		t.Fatalf("retry did not acknowledge delivery: calls=%d job=%+v", calls, job)
	}
}

func TestCollectibleLifecycleDeliveryRetriesReservationRelease(t *testing.T) {
	db, err := repo.MockDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	jobID := models.CollectibleLifecycleRelease + ":release-order"
	if err := db.Update(func(tx database.Tx) error {
		if err := tx.Migrate(&models.CollectibleLifecycleDelivery{}); err != nil {
			return err
		}
		return tx.Save(&models.CollectibleLifecycleDelivery{
			JobID: jobID, OrderID: "release-order", Kind: models.CollectibleLifecycleRelease, Reason: "order cancelled",
		})
	}); err != nil {
		t.Fatal(err)
	}

	calls := 0
	node := &MobazhaNode{
		storageFields: storageFields{db: db},
		collectiblesFields: collectiblesFields{
			collectibleFirstSaleReservationReleaseHook: func(_ context.Context, signal CollectibleFirstSaleReservationReleaseSignal) error {
				calls++
				if signal.OrderID != "release-order" || signal.Reason != "order cancelled" {
					t.Fatalf("unexpected release signal: %+v", signal)
				}
				if calls == 1 {
					return errors.New("temporary release failure")
				}
				return nil
			},
		},
	}
	node.runCollectibleLifecycleDeliveries(context.Background())
	past := time.Now().UTC().Add(-time.Second)
	if err := db.Update(func(tx database.Tx) error {
		return tx.Update("next_attempt_at", past, map[string]interface{}{"job_id = ?": jobID}, &models.CollectibleLifecycleDelivery{})
	}); err != nil {
		t.Fatal(err)
	}
	node.runCollectibleLifecycleDeliveries(context.Background())

	var job models.CollectibleLifecycleDelivery
	if err := db.View(func(tx database.Tx) error {
		return tx.Read().Where("job_id = ?", jobID).First(&job).Error
	}); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || job.DeliveredAt == nil || job.Attempts != 2 {
		t.Fatalf("release job was not retried to completion: calls=%d job=%+v", calls, job)
	}
}
