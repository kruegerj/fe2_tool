package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sys/windows/registry"
	"gopkg.in/yaml.v2"
)

type Config struct {
	MongoDB struct {
		Database   string `yaml:"database"`
		Collection string `yaml:"collection"`
	} `yaml:"mongodb"`
}

type User struct {
	Name string `bson:"name"`
	Unit []struct {
		Name string `bson:"name"`
		Type string `bson:"type"`
		Code string `bson:"code"`
	} `bson:"units"`
}

func main() {
	// Lese die YAML-Datei
	yamlFile, err := os.ReadFile("settings.yml")
	if err != nil {
		log.Fatalf("Fehler beim Lesen der YAML-Datei: %v", err)
	}

	// Deklariere eine Instanz der Config-Struktur
	var config Config

	// Unmarshal der YAML-Daten in die Config-Struktur
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		log.Fatalf("Fehler beim Unmarshaling der YAML-Daten: %v", err)
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\JavaSoft\Prefs\de.alamos.fe2.server.services./Registry/Service`, registry.QUERY_VALUE)
	if err != nil {
		log.Fatal(err)
	}
	defer k.Close()

	pw, _, err := k.GetStringValue("dbpassword")
	if err != nil {
		log.Fatal(err)
	}
	log.Infof("Windows system root is %q\n", pw)
	// Schreibgeschützte MongoDB-Verbindung erstellen
	client, err := createReadOnlyMongoDBClient(pw)
	if err != nil {
		log.Errorf("Fehler bei der Verbindung zur MongoDB: %s", err)
		return
	}
	defer client.Disconnect(context.TODO())

	// Datenbank und Sammlung auswählen
	database := client.Database(config.MongoDB.Database)
	collection := database.Collection(config.MongoDB.Collection)

	// Projektion erstellen, um die gewünschten Felder auszuwählen
	//projection := options.Find().SetProjection(map[string]interface{}{"name": 1, "unit.$[].name": 1})

	// Abfrage erstellen und die gewünschte Projektion verwenden
	// Schreibgeschützte Abfrage durchführen
	cursor, err := collection.Find(context.TODO(), bson.M{})
	if err != nil {
		log.Errorln("Fehler bei der Abfrage:", err)
		return
	}
	defer cursor.Close(context.TODO())
	// Aktuelles Datum und Uhrzeit für Dateinamen erstellen
	currentTime := time.Now()
	dateTimeStr := currentTime.Format("2006-01-02_15-04-05") // Format: JJJJ-MM-TT_Std-Min-Sek

	// CSV-Datei erstellen
	fileName := fmt.Sprintf("exported_data_%s.csv", dateTimeStr)

	file, err := os.Create(fileName)
	if err != nil {
		log.Errorln("Fehler beim Erstellen der CSV-Datei:", err)
		return
	}
	defer file.Close()
	// CSV-Schreiber erstellen
	writer := csv.NewWriter(file)
	defer writer.Flush()
	writer.Comma = ';'
	writer.UseCRLF = true // Zeilenumbruch für Windows

	// Überschriftenzeile schreiben
	writer.Write([]string{"User", "Name", "Code", "Type"})
	// Ergebnisse verarbeiten
	var users []User
	for cursor.Next(context.TODO()) {
		var user User
		if err := cursor.Decode(&user); err != nil {
			log.Errorf("Fehler beim Dekodieren des Ergebnisses: %s", err)
			return
		}
		users = append(users, user)
	}

	// Ergebnisse ausgeben
	for _, user := range users {
		//log.Infof("Name: %s", user.Name)
		for _, unit := range user.Unit {
			writer.Write([]string{user.Name, unit.Name, unit.Code, unit.Type})
			log.Infof("User Name: %s;Unit Name: %s;Unit Code: %s", user.Name, unit.Name, unit.Code)
		}
	}

	if err := cursor.Err(); err != nil {
		log.Errorf("Fehler bei der Abfrage: %s", err)
		return
	}

}

func createReadOnlyMongoDBClient(pw string) (*mongo.Client, error) {
	// Verbindungsoptionen für die MongoDB-Verbindung konfigurieren
	clientOptions := options.Client().ApplyURI("mongodb://Admin:" + pw + "@localhost:27018/")

	// Schreibgeschützte Verbindung erstellen
	client, err := mongo.NewClient(clientOptions)
	if err != nil {
		return nil, err
	}

	// Kontext erstellen
	ctx := context.TODO()

	// Verbindung zur MongoDB herstellen
	err = client.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return client, nil
}
