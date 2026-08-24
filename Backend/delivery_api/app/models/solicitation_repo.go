package models

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"

	"github.com/carloshomar/fuudelivery/delivery_api/app/dto"
)

// collectionSolicitations é a collection Mongo que permanece como espelho
// durante o corte (dual-write). Depois do corte, pode ser descartada.
func collectionSolicitations() *mongo.Collection {
	return MongoDabase.Collection("solicitations")
}

// UpsertSolicitation insere ou atualiza (por order_id) em Postgres E Mongo.
// Retorna o registro persistido (com o deliveryman já atribuído, se houver)
// e se o pedido já existia (update vs insert). Na atualização preserva o
// deliveryman atribuído — só o status muda, igual ao comportamento Mongo.
func UpsertSolicitation(ctx context.Context, order dto.OrderDTO) (dto.OrderDTO, bool, error) {
	existed := false
	var stored dto.OrderDTO

	// ── Postgres (primário) ──
	if DB != nil {
		rec := FromOrderDTO(order)
		var existing DeliverySolicitation
		err := DB.WithContext(ctx).Where("order_id = ?", order.OrderId).First(&existing).Error
		if err == nil {
			existed = true
			updates := map[string]interface{}{
				"status":     order.Status,
				"updated_at": time.Now(),
			}
			if err := DB.WithContext(ctx).Model(&DeliverySolicitation{}).
				Where("order_id = ?", order.OrderId).Updates(updates).Error; err != nil {
				log.Printf("[DELIVERY_PG] UpsertSolicitation update %s: %v", order.OrderId, err)
			}
			stored = existing.ToOrderDTO()
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[DELIVERY_PG] UpsertSolicitation find %s: %v", order.OrderId, err)
		} else {
			if err := DB.WithContext(ctx).Create(&rec).Error; err != nil {
				log.Printf("[DELIVERY_PG] UpsertSolicitation insert %s: %v", order.OrderId, err)
			}
			stored = rec.ToOrderDTO()
		}
	}

	// ── Mongo (espelho, preserva o comportamento atual) ──
	if MongoDabase != nil {
		coll := collectionSolicitations()
		var existing dto.OrderDTO
		err := coll.FindOne(ctx, bson.M{"orderid": order.OrderId}).Decode(&existing)
		if err == nil {
			existed = true
			update := bson.M{"$set": bson.M{"status": order.Status, "operationDate": time.Now()}}
			if _, err := coll.UpdateOne(ctx, bson.M{"orderid": order.OrderId}, update); err != nil {
				log.Printf("[DELIVERY_MONGO] UpsertSolicitation update %s: %v", order.OrderId, err)
			}
			stored = existing // preserva o deliveryman já atribuído
		} else if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("[DELIVERY_MONGO] UpsertSolicitation find %s: %v", order.OrderId, err)
		} else {
			if _, err := coll.InsertOne(ctx, order); err != nil {
				log.Printf("[DELIVERY_MONGO] UpsertSolicitation insert %s: %v", order.OrderId, err)
			}
			stored = order
		}
	}

	if DB == nil && MongoDabase == nil {
		return stored, existed, fmt.Errorf("nenhuma fonte de dados configurada (Postgres/Mongo)")
	}
	return stored, existed, nil
}

// UpdateSolicitationDeliveryMan atribui (ou substitui) o deliveryman de um
// pedido em Postgres e Mongo (handshake).
func UpdateSolicitationDeliveryMan(ctx context.Context, orderID string, dm dto.DeliveryManDTO) error {
	if DB != nil {
		var dmID *int64
		if dm.Id != 0 {
			v := dm.Id
			dmID = &v
		}
		updates := map[string]interface{}{
			"delivery_man_id":     dmID,
			"delivery_man_name":   dm.Name,
			"delivery_man_status": dm.Status,
			"updated_at":          time.Now(),
		}
		if err := DB.WithContext(ctx).Model(&DeliverySolicitation{}).
			Where("order_id = ?", orderID).Updates(updates).Error; err != nil {
			log.Printf("[DELIVERY_PG] UpdateSolicitationDeliveryMan %s: %v", orderID, err)
			return err
		}
	}
	if MongoDabase != nil {
		update := bson.M{"$set": bson.M{"deliveryman": dm}}
		if _, err := collectionSolicitations().UpdateOne(ctx, bson.M{"orderid": orderID}, update); err != nil {
			log.Printf("[DELIVERY_MONGO] UpdateSolicitationDeliveryMan %s: %v", orderID, err)
			return err
		}
	}
	return nil
}

// UpdateSolicitationDeliveryManStatus atualiza apenas o status do deliveryman
// de um pedido, restringindo pelo ID do entregador (autorização). Retorna
// false se o pedido não existe ou o entregador não é o dono.
func UpdateSolicitationDeliveryManStatus(ctx context.Context, orderID string, dmID int64, status string) (bool, error) {
	found := false

	if DB != nil {
		res := DB.WithContext(ctx).Model(&DeliverySolicitation{}).
			Where("order_id = ? AND delivery_man_id = ?", orderID, dmID).
			Updates(map[string]interface{}{
				"delivery_man_status": status,
				"updated_at":          time.Now(),
			})
		if res.Error != nil {
			log.Printf("[DELIVERY_PG] UpdateSolicitationDeliveryManStatus %s: %v", orderID, res.Error)
			return false, res.Error
		}
		if res.RowsAffected > 0 {
			found = true
		}
	}

	if MongoDabase != nil {
		filter := bson.M{"orderid": orderID, "deliveryman.id": dmID}
		update := bson.M{"$set": bson.M{"deliveryman.status": status}}
		res, err := collectionSolicitations().UpdateOne(ctx, filter, update)
		if err != nil {
			log.Printf("[DELIVERY_MONGO] UpdateSolicitationDeliveryManStatus %s: %v", orderID, err)
			return false, err
		}
		if res.MatchedCount > 0 {
			found = true
		}
	}

	return found, nil
}

// GetSolicitationByOrderID busca um pedido pelo order_id. Lê do Postgres
// (primário) e cai para o Mongo durante o corte quando não encontra lá.
func GetSolicitationByOrderID(ctx context.Context, orderID string) (*dto.OrderDTO, error) {
	if DB != nil {
		var rec DeliverySolicitation
		err := DB.WithContext(ctx).Where("order_id = ?", orderID).First(&rec).Error
		if err == nil {
			out := rec.ToOrderDTO()
			return &out, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[DELIVERY_PG] GetSolicitationByOrderID %s: %v", orderID, err)
		}
	}
	if MongoDabase != nil {
		var order dto.OrderDTO
		err := collectionSolicitations().FindOne(ctx, bson.M{"orderid": orderID}).Decode(&order)
		if err == nil {
			return &order, nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("[DELIVERY_MONGO] GetSolicitationByOrderID %s: %v", orderID, err)
		}
	}
	return nil, fmt.Errorf("solicitação %s não encontrada", orderID)
}

// GetDeliveryManIDByOrderID retorna o ID do deliveryman atribuído a um
// pedido (usado nos checks de autorização do chat/monolito). 0 = sem
// entregador atribuído.
func GetDeliveryManIDByOrderID(ctx context.Context, orderID string) (int64, error) {
	if DB != nil {
		var rec DeliverySolicitation
		err := DB.WithContext(ctx).Select("delivery_man_id").Where("order_id = ?", orderID).First(&rec).Error
		if err == nil {
			if rec.DeliveryManID != nil {
				return *rec.DeliveryManID, nil
			}
			return 0, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("[DELIVERY_PG] GetDeliveryManIDByOrderID %s: %v", orderID, err)
		}
	}
	if MongoDabase != nil {
		var doc struct {
			DeliveryMan dto.DeliveryManDTO `bson:"deliveryman"`
		}
		if err := collectionSolicitations().FindOne(ctx, bson.M{"orderid": orderID}).Decode(&doc); err == nil {
			return doc.DeliveryMan.Id, nil
		}
	}
	return 0, fmt.Errorf("solicitação %s não encontrada", orderID)
}

// FindApprovedSolicitations retorna pedidos aprovados/concluídos ainda sem
// entregador atribuído (fila aberta para os entregadores).
func FindApprovedSolicitations(ctx context.Context) ([]dto.OrderDTO, error) {
	if DB != nil {
		var recs []DeliverySolicitation
		err := DB.WithContext(ctx).
			Where("status IN ? AND (delivery_man_id IS NULL OR delivery_man_id = 0)", []string{"APPROVED", "DONE"}).
			Find(&recs).Error
		if err == nil {
			out := make([]dto.OrderDTO, 0, len(recs))
			for _, r := range recs {
				out = append(out, r.ToOrderDTO())
			}
			return out, nil
		}
		log.Printf("[DELIVERY_PG] FindApprovedSolicitations: %v", err)
	}
	if MongoDabase != nil {
		filter := bson.M{
			"status": bson.M{"$in": []string{"APPROVED", "DONE"}},
			"$or": []bson.M{
				{"deliveryman": nil},
				{"deliveryman.id": 0},
			},
		}
		cur, err := collectionSolicitations().Find(ctx, filter)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		var out []dto.OrderDTO
		for cur.Next(ctx) {
			var o dto.OrderDTO
			if err := cur.Decode(&o); err != nil {
				continue
			}
			out = append(out, o)
		}
		return out, nil
	}
	return nil, fmt.Errorf("nenhuma fonte de dados configurada (Postgres/Mongo)")
}

// FindActiveOrdersByDeliveryman retorna os pedidos ativos de um entregador
// (status do pedido e do entregador diferentes de FINISHED).
func FindActiveOrdersByDeliveryman(ctx context.Context, dmID int64) ([]dto.OrderDTO, error) {
	if DB != nil {
		var recs []DeliverySolicitation
		err := DB.WithContext(ctx).
			Where("delivery_man_id = ? AND status <> 'FINISHED' AND delivery_man_status <> 'FINISHED'", dmID).
			Order("created_at DESC").
			Find(&recs).Error
		if err == nil {
			out := make([]dto.OrderDTO, 0, len(recs))
			for _, r := range recs {
				out = append(out, r.ToOrderDTO())
			}
			return out, nil
		}
		log.Printf("[DELIVERY_PG] FindActiveOrdersByDeliveryman %d: %v", dmID, err)
	}
	if MongoDabase != nil {
		filter := bson.M{
			"deliveryman.id":     dmID,
			"status":             bson.M{"$ne": "FINISHED"},
			"deliveryman.status": bson.M{"$ne": "FINISHED"},
		}
		cur, err := collectionSolicitations().Find(ctx, filter)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		var out []dto.OrderDTO
		for cur.Next(ctx) {
			var o dto.OrderDTO
			if err := cur.Decode(&o); err != nil {
				continue
			}
			out = append(out, o)
		}
		return out, nil
	}
	return nil, fmt.Errorf("nenhuma fonte de dados configurada (Postgres/Mongo)")
}

// FindFinishedOrdersByDeliveryman retorna o extrato (pedidos FINISHED) de um
// entregador, do mais recente para o mais antigo.
func FindFinishedOrdersByDeliveryman(ctx context.Context, dmID int64) ([]dto.OrderDTO, error) {
	if DB != nil {
		var recs []DeliverySolicitation
		err := DB.WithContext(ctx).
			Where("delivery_man_id = ? AND status = 'FINISHED' AND delivery_man_status = 'FINISHED'", dmID).
			Order("created_at DESC").
			Find(&recs).Error
		if err == nil {
			out := make([]dto.OrderDTO, 0, len(recs))
			for _, r := range recs {
				out = append(out, r.ToOrderDTO())
			}
			return out, nil
		}
		log.Printf("[DELIVERY_PG] FindFinishedOrdersByDeliveryman %d: %v", dmID, err)
	}
	if MongoDabase != nil {
		filter := bson.M{
			"deliveryman.id":     dmID,
			"status":             "FINISHED",
			"deliveryman.status": "FINISHED",
		}
		opts := options.Find().SetSort(bson.D{{Key: "operationDate", Value: -1}})
		cur, err := collectionSolicitations().Find(ctx, filter, opts)
		if err != nil {
			return nil, err
		}
		defer cur.Close(ctx)
		var out []dto.OrderDTO
		for cur.Next(ctx) {
			var o dto.OrderDTO
			if err := cur.Decode(&o); err != nil {
				continue
			}
			out = append(out, o)
		}
		return out, nil
	}
	return nil, fmt.Errorf("nenhuma fonte de dados configurada (Postgres/Mongo)")
}
