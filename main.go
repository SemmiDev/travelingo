package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"net/http"
	"os"
	"time"
)

// load configuration
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
}

// IsProduction for app environment
func IsProduction() bool {
	return os.Getenv("APP_PROFILE") == "production"
}

// Travel for field represent in table
type Travel struct {
	ObjectID primitive.ObjectID `json:"id" bson:"_id"`
	Name 	string `json:"name" bson:"name"`
	Photo 	string `json:"photo" bson:"photo"`
	Done 	bool `json:"done" bson:"done"`
}

// Travels for Travel slices
type Travels = []Travel

// DBRepository for Travel repository
type DBRepository struct {
	client 		*mongo.Client
	database	*mongo.Database
	Collection 	*mongo.Collection
}

// Repository for Travel repository interfaces
type Repository interface {
	ping() (string, error)
	findAll(ctx context.Context) (*Travels, error)
	findOne(ctx context.Context, id string) (*Travel, error)
	insertOne(ctx context.Context, travel *Travel) error
	updateOne(ctx context.Context, id string, travel *Travel) error
	updateField(ctx context.Context, id, field string, value interface{}) error
	deleteOne(ctx context.Context, id string) error
	Close()
}

// NewRepo for Travel Repository initialize
func NewRepo(uri string) (Repository, error) {
	client, err := mongo.NewClient(options.Client().ApplyURI(uri))
	log.Println("db client created")
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20 * time.Second)
	defer cancel()
	err = client.Connect(ctx)

	if err != nil {
		return nil, err
	}
	log.Println("db client connected")

	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		return nil, err
	}
	log.Println("db client ping")

	dbName := os.Getenv("DATABASE_NAME")
	db := client.Database(dbName)
	col := db.Collection(os.Getenv("TRAVEL_COLLECTION"))
	return &DBRepository{
		client: 	client,
		database:   db,
		Collection: col,
	}, nil
}

// ping() for check connection is established?
func (d *DBRepository) ping() (string, error) {
	ctx := context.Background()
	err := d.client.Ping(ctx, readpref.Primary())
	if err != nil {
		return "", errors.New("connection error")
	}
	return "connection to database established", nil
}

// findAll() for find all travels
func (d *DBRepository) findAll(ctx context.Context) (*Travels, error) {
	c, err := d.Collection.Find(ctx, bson.D{})
	if err != nil {
		return nil, err
	}
	var travels Travels

	for c.Next(ctx) {
		var travel Travel
		if err := c.Decode(&travel); err != nil {
			return nil, err
		}
		travels = append(travels, travel)
	}
	if err := c.Close(ctx); err != nil {
		return nil, err
	}
	return &travels, nil
}

// findOne() for find a travel
func (d *DBRepository) findOne(ctx context.Context, id string) (*Travel, error) {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	res := d.Collection.FindOne(ctx, bson.M{"_id": objectId})
	var travel Travel
	if err := res.Decode(&travel); err != nil {
		return nil, err
	}
	return &travel, nil
}

// insertOne() for insert a data to collection
func (d *DBRepository) insertOne(ctx context.Context, travel *Travel) error {
	travel.ObjectID = primitive.NewObjectID()
	if _, err := d.Collection.InsertOne(ctx, travel); err != nil {
		return err
	}
	return nil
}

// updateOne() for update a data in collection
func (d *DBRepository) updateOne(ctx context.Context, id string, travel *Travel) error {
	travel.ObjectID, _ = primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": travel.ObjectID}
	if _, err := d.Collection.ReplaceOne(ctx, filter, travel); err != nil {
		return err
	}
	return nil
}

// updateField() for update a field
func (d *DBRepository) updateField(ctx context.Context, id, field string, value interface{}) error {
	objectID, _ := primitive.ObjectIDFromHex(id)
	filter := bson.M{"_id": objectID}
	update := bson.D{{
		"$set", bson.D{{
			field, value,
		}},
	}}
	if _, err := d.Collection.ReplaceOne(ctx, filter, update); err != nil {
		return err
	}
	return nil
}

// deleteOne() for delete a data from coll
func (d *DBRepository) deleteOne(ctx context.Context, id string) error {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	if _, err := d.Collection.DeleteOne(ctx, bson.M{"_id": objectId}); err != nil {
		return err
	}
	return nil
}

// Close Close() for close connection
func (d *DBRepository) Close() {
	if err := d.client.Disconnect(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// appService struct for Travel repository
type appService struct {
	Repository Repository
}

// Service for Travel service interfaces
type Service interface {
	getTravels(c *fiber.Ctx) error
	getTravel(c *fiber.Ctx) error
	createTravel(c *fiber.Ctx) error
	updateTravel(c *fiber.Ctx) error
	deleteTravel(c *fiber.Ctx) error
}

// NewService for initialize service
func NewService(r Repository) Service {
	return &appService{Repository: r}
}

// getTravels() for get Travels
func (a *appService) getTravels(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)

	defer cancel()

	travels, err := a.Repository.findAll(ctx)
	return response(travels, http.StatusOK, err, c)
}

// getTravel() for get a Travel
func (a *appService) getTravel(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return response(nil, http.StatusUnprocessableEntity, errors.New("id is not defined"), c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	travel, err := a.Repository.findOne(ctx, id)
	return response(travel, http.StatusOK, err, c)
}

// getTravel() for create a Travel
func (a *appService) createTravel(c *fiber.Ctx) error {
	var travel Travel
	if err := c.BodyParser(&travel); err != nil {
		return response(travel, http.StatusUnprocessableEntity, err, c)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := a.Repository.insertOne(ctx, &travel)
	return response(travel, http.StatusOK, err, c)
}

// updateTravel() for update a Travel
func (a *appService) updateTravel(c *fiber.Ctx) error {
	id := c.Params("id")
	log.Println(id)
	if id == "" {
		return response(nil, http.StatusUnprocessableEntity, errors.New("id is not defined"), c)
	}
	var travel Travel
	if err := c.BodyParser(&travel); err != nil {
		return response(travel, http.StatusUnprocessableEntity, err, c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := a.Repository.updateOne(ctx, id, &travel)
	return response(nil, http.StatusNoContent, err, c)
}

// deleteTravel() for delete a travel
func (a *appService) deleteTravel(c *fiber.Ctx) error {
	id := c.Params("id")
	log.Println(id)
	if id == "" {
		return response(nil, http.StatusUnprocessableEntity, errors.New("id is not defined"), c)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	err := a.Repository.deleteOne(ctx, id)
	return response(nil, http.StatusNoContent, err, c)
}

// response to route
func response(data interface{}, httpStatus int, err error, c *fiber.Ctx) error {
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(map[string]string{
			"error": err.Error(),
		})
	} else {
		if data != nil {
			return c.Status(httpStatus).JSON(data)
		} else {
			c.Status(httpStatus)
			return nil
		}
	}
}

// Routes for endpoint to access handler
func Routes(app *fiber.App, service Service) {
	api := app.Group("/api/v1")
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.Status(http.StatusOK).
			JSON(map[string]interface{}{
				"health": "ok",
				"status": http.StatusOK,
			})
	})

	api.Get("/travels", service.getTravels)
	api.Get("/travels/:id", service.getTravel)
	api.Post("/travels", service.createTravel)
	api.Put("/travels/:id", service.updateTravel)
	api.Delete("/travels/:id", service.deleteTravel)
}

// yeah!! GO
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Running application in %v environment", os.Getenv("APP_PROFILE"))
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run() for initialize fiber app
func run() error {
	port := os.Getenv("PORT")
	dbURI := os.Getenv("DATABASE_URI")

	// conn -> repo
	r, err := NewRepo(dbURI)
	if err != nil {
		log.Fatal(err)
	}

	defer r.Close()

	// repo -> service
	service := NewService(r)

	// fiber initialize
	app := fiber.New()
	if !IsProduction() {
		app.Use(logger.New())
		app.Use(cors.New())
	}

	// service -> routes
	Routes(app, service)
	return app.Listen(fmt.Sprintf(":%s", port))
}