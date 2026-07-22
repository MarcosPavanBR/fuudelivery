package repository

import (
	"time"

	"github.com/carloshomar/vercardapio/payment/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

func GetUserByEmail(email string) (*models.User, error) {
	ctx := MongoCtx()
	var user models.User
	err := Users.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func GetUserByID(id string) (*models.User, error) {
	ctx := MongoCtx()
	var user models.User
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	err = Users.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func CreateUser(user *models.User) error {
	ctx := MongoCtx()
	user.ID = primitive.NewObjectID().Hex()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hashedPassword)

	_, err = Users.InsertOne(ctx, user)
	return err
}

func ListUsers() ([]models.User, error) {
	ctx := MongoCtx()
	cursor, err := Users.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func UpdateUserPassword(email string, newPassword string) error {
	ctx := MongoCtx()
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = Users.UpdateOne(ctx,
		bson.M{"email": email},
		bson.M{"$set": bson.M{"password": string(hashedPassword)}},
	)
	return err
}
