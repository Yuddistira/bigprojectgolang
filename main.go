package main

import (
	"database/sql"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"time"

	redigo "github.com/garyburd/redigo/redis"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	nsq "github.com/nsqio/go-nsq"
)

var (
	srv       = &http.Server{Addr: ":8080"}
	dbHome    *sql.DB
	err       error
	redisPool *redigo.Pool
	redisKey  = "visitors"
	configNsq *nsq.Config
)

type User struct {
	ID          int
	Name        sql.NullString
	MSISDN      sql.NullString
	Email       string
	BirthDate   pq.NullTime
	CreateTime  pq.NullTime
	UpdateTime  pq.NullTime
	UserAge     sql.NullString
	Calculation sql.NullString
}

func main() {
	log.Printf("main: initialize open DB")
	initializeDB()
	if dbHome != nil {

	}
	defer dbHome.Close()
	redisPool = NewRedis("localhost:6379")
	configNsq = nsq.NewConfig()

	log.Printf("main: starting http server")
	http.HandleFunc("/", HomePage)
	http.HandleFunc("/v1/getUser", getUserHandler)
	http.HandleFunc("/v1/getVisitor", getVisitorHandler)

	testConsumer()
	if err := srv.ListenAndServe(); err != nil {
		log.Printf("Httpserver: ListenAndServe() error : %s ", err)
	}
}

func getUserHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("nameInput")
	userlist2 := getMultipleCategoryWithStatement(query)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	result, _ := json.Marshal(userlist2)
	w.Write(result)
}

func getVisitorHandler(w http.ResponseWriter, r *http.Request) {
	var visitor int

	if val, err := GetRedis(redisKey); err != nil {
		log.Println(err)
	} else {
		visitor = val
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	result, _ := json.Marshal(visitor)
	w.Write(result)
}

func initializeDB() {
	dbHome, err = sql.Open("postgres", "postgres://yk180402:eequoo0fee1ve9waew0I@devel-postgre.tkpd/tokopedia-user?sslmode=disable")
	if err != nil {
		log.Print(err)
	}
}

func getMultipleCategoryWithStatement(nameInput string) []User {
	stmt, err := dbHome.Prepare("SELECT user_id, full_name, msisdn, user_email,birth_date,create_time, update_time FROM ws_user WHERE birth_date IS NOT NULL and lower(full_name) like '%" + nameInput + "%'  LIMIT 10 ")
	userlist := []User{}

	rows, err := stmt.Query()
	if err != nil {
		log.Print(err)
		return userlist
	}

	defer rows.Close()
	for rows.Next() {
		u := &User{}
		rows.Scan(
			&u.ID,
			&u.Name,
			&u.MSISDN,
			&u.Email,
			&u.BirthDate,
			&u.CreateTime,
			&u.UpdateTime,
		)

		userlist = append(userlist, *u)
	}
	testProducer()
	return userlist
}

func HomePage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("homepage.html")
	if err != nil {
		log.Print("template parsing error: ", err)
	}
	err = t.Execute(w, nil)
	if err != nil {
		log.Print("template executing error: ", err)
	}
}

//redis code
func NewRedis(address string) *redigo.Pool {
	return &redigo.Pool{
		MaxIdle:     1,
		IdleTimeout: 10 * time.Second,
		Dial:        func() (redigo.Conn, error) { return redigo.Dial("tcp", address) },
	}
}

func SetRedis(key string, value int) error {
	con := redisPool.Get()
	defer con.Close()

	_, err := con.Do("SET", key, value)
	return err
}

func GetRedis(key string) (int, error) {
	con := redisPool.Get()
	defer con.Close()

	return redigo.Int(con.Do("GET", key))
}

func incrementVisitor(key string) error {
	con := redisPool.Get()
	defer con.Close()

	_, err := con.Do("INCR", key)
	return err
}

//nsq code
func testProducer() {
	w, _ := nsq.NewProducer("devel-go.tkpd:4150", configNsq)

	err := w.Publish("bp-yuddis", []byte("incr"))
	if err != nil {
		log.Panic("Could not connect")
	}

	w.Stop()
}

func testConsumer() {
	q, err := nsq.NewConsumer("bp-yuddis", "ch", configNsq)
	if err != nil {
		log.Fatal("failed to create consumer ", err)
	}
	q.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		if err := incrementVisitor(redisKey); err != nil {
			log.Print(err)
		}
		return nil
	}))
	q.ConnectToNSQLookupd("devel-go.tkpd:4161")
}
