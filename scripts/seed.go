//go:build ignore

package main

import (
    "log"
    "os"
    "github.com/joho/godotenv"
    "golang.org/x/crypto/bcrypt"
    authModels "github.com/carloshomar/vercardapio/auth_api/app/models"
    ordersModels "github.com/carloshomar/vercardapio/orders_api/app/models"
)

func main() {
    godotenv.Load()
    
    // Connect to databases
    authModels.ConnectDatabase()
    ordersModels.ConnectPostgresDatabase()
    
    log.Println("Seeding data...")
    
    // Create admin user
    hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
    admin := authModels.User{
        Name:     "Admin Fuudelivery",
        Email:    "admin@fuudelivery.com",
        Password: string(hashedPassword),
    }
    authModels.DB.Create(&admin)
    
    // Create establishment
    est := authModels.Establishment{
        Name:                "Fuudelivery Demo",
        Description:         "Restaurante demonstrativo",
        OwnerID:             admin.ID,
        Image:               "https://via.placeholder.com/200",
        PrimaryColor:        "#F97316",
        SecondaryColor:      "#FCD34D",
        Lat:                 -23.5505,
        Long:                -46.6333,
        MaxDistanceDelivery: 10,
        LocationString:      "São Paulo, SP",
    }
    authModels.DB.Create(&est)
    
    // Create business hours
    for day := 0; day < 7; day++ {
        bh := authModels.BusinessHours{
            EstablishmentID: est.ID,
            DayOfWeek:       day,
            IsOpen:          day != 0, // closed on Sunday
            OpenTime:        "08:00",
            CloseTime:       "22:00",
        }
        authModels.DB.Create(&bh)
    }
    
    // Create categories
    categories := []ordersModels.Category{
        {Name: "Hambúrgueres", Image: "https://via.placeholder.com/100", EstablishmentID: est.ID},
        {Name: "Bebidas", Image: "https://via.placeholder.com/100", EstablishmentID: est.ID},
        {Name: "Sobremesas", Image: "https://via.placeholder.com/100", EstablishmentID: est.ID},
    }
    for _, cat := range categories {
        ordersModels.DB.Create(&cat)
    }
    
    // Create products
    products := []ordersModels.Product{
        {Name: "X-Burger", Description: "Hambúrguer com queijo e alface", Price: 25.90, Image: "https://via.placeholder.com/200", EstablishmentID: est.ID},
        {Name: "X-Salada", Description: "Hambúrguer com queijo, alface e tomate", Price: 28.90, Image: "https://via.placeholder.com/200", EstablishmentID: est.ID},
        {Name: "Coca-Cola 350ml", Description: "Refrigerante de cola", Price: 6.90, Image: "https://via.placeholder.com/200", EstablishmentID: est.ID},
        {Name: "Suco de Laranja", Description: "Suco natural 500ml", Price: 8.90, Image: "https://via.placeholder.com/200", EstablishmentID: est.ID},
        {Name: "Pudim", Description: "Pudim de leite condensado", Price: 12.90, Image: "https://via.placeholder.com/200", EstablishmentID: est.ID},
    }
    for _, prod := range products {
        ordersModels.DB.Create(&prod)
    }
    
    log.Println("✓ Seed data created!")
    log.Println("  Admin email: admin@fuudelivery.com")
    log.Println("  Admin password: admin123")
}
