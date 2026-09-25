package models

type Product struct {
	ID              uint   `gorm:"primaryKey"`
	Name            string `gorm:"not null"`
	Description     string
	Price           float64 `gorm:"not null"`
	Image           string
	EstablishmentID uint
	// Available=false é o item pausado/esgotado: continua no cardápio da
	// loja, aparece como "Esgotado" para o cliente e o pedido com ele é
	// recusado em computeOrderTotal. Muda só por PUT /products/:id/availability
	// (Save com struct pularia o false por causa do default).
	Available  bool         `gorm:"not null;default:true"`
	Categories []Category   `gorm:"many2many:category_products;foreignKey:ID;joinForeignKey:ProductID;joinReferences:CategoryID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Additional []Additional `gorm:"many2many:additional_products;foreignKey:ID;joinForeignKey:ProductID;joinReferences:AdditionalID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
}
