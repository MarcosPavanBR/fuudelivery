package dto

import (
	"github.com/carloshomar/fuudelivery/orders_api/app/models"
)

type CategorieRequest struct {
	Id              uint             `json:"ID"`
	Name            string           `json:"Name"`
	Image           string           `json:"Image"`
	EstablishmentId uint             `json:"EstablishmentId"`
	Products        []models.Product `json:"Products"`
}
