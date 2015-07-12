package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"path"

	"github.com/boltdb/bolt"
	"github.com/clbanning/mxj"
	"github.com/hashicorp/memberlist"
	"github.com/julienschmidt/httprouter"
	"github.com/pokstad/go-couchdb"
)

type jsonStruct struct {
	Path string `json:"path"`
}

type member struct {
	Name string `json:"name"`
	Addr net.IP `json:"addr"`
	Port uint16 `json:"port"`
}

type file struct {
	UUID   uuid.UUID `json:"uuid"`
	Fname  string    `json:"fname"`
	UserID string    `json:"userid"`
}

type userFiles struct {
	TotalFiles int `json:"total_rows"`
	Rows       []struct {
		Key string `json:"key"`
		Doc struct {
			ID     string    `json:"_id"`
			UUID   uuid.UUID `json:"uuid"`
			Fname  string    `json:"fname"`
			UserID string    `json:"userid"`
		} `json:"doc"`
	} `json:"rows"`
}

type userFilesActive struct {
	Rows []struct {
		Key string `json:"key"`
		Doc struct {
			ID     string    `json:"_id"`
			UUID   uuid.UUID `json:"uuid"`
			Fname  string    `json:"fname"`
			UserID string    `json:"userid"`
		} `json:"value"`
	} `json:"rows"`
}

// GetActiveFiles gets active files
func GetActiveFiles(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	resp, err := http.Get("http://localhost:3000/members")
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}

	defer resp.Body.Close()
	members := []member{}
	contents, _ := ioutil.ReadAll(resp.Body)
	if err := json.Unmarshal(contents, &members); err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
	}
	var memberNames []string
	for _, memberDat := range members {
		memberNames = append(memberNames, memberDat.Name)
	}

	memberJSON, _ := json.Marshal(memberNames)
	query := url.QueryEscape(string(memberJSON))
	resp, err = http.Get("http://localhost:5984/files/_design/design1/_view/userFiles?keys=" + query)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		fmt.Fprint(w, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var uf userFilesActive

	json.Unmarshal(body, &uf)

	w.WriteHeader(200)
	ufJSON, _ := json.Marshal(uf)
	m, _ := mxj.NewMapJson(ufJSON)
	xmlVal, _ := m.Xml()
	fmt.Fprint(w, string(xmlVal))
}

// GetAllFiles gets all files, active or not
func GetAllFiles(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	fmt.Println("LOL")
	resp, err := http.Get("http://localhost:5984/files/_all_docs?include_docs=true")
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		fmt.Fprint(w, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var uf userFiles

	json.Unmarshal(body, &uf)

	w.WriteHeader(200)
	ufJSON, _ := json.Marshal(uf)
	m, _ := mxj.NewMapJson(ufJSON)
	xmlVal, _ := m.Xml()
	fmt.Fprint(w, string(xmlVal))
}

// GetMemberFiles get a member's files
func GetMemberFiles(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	query := url.QueryEscape(`"` + p.ByName("name") + `"`)
	reqURL := "http://localhost:5984/files/_design/design1/_view/userFiles?key=" + query
	resp, err := http.Get(reqURL)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		fmt.Fprint(w, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var uf userFilesActive

	json.Unmarshal(body, &uf)

	w.WriteHeader(200)
	ufJSON, _ := json.Marshal(uf)
	m, _ := mxj.NewMapJson(ufJSON)
	xmlVal, _ := m.Xml()
	fmt.Fprint(w, string(xmlVal))
}

// GetMembers gets the list of members by redirecting to Scatter endpoint
func GetMembers(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	resp, err := http.Get("http://localhost:3000")
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		fmt.Fprint(w, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	var members []member
	json.Unmarshal(body, &members)
	w.WriteHeader(200)
	ufJSON, _ := json.Marshal(members)
	m, _ := mxj.NewMapJson(ufJSON)
	xmlVal, _ := m.Xml()
	fmt.Fprint(w, string(xmlVal))
}

// ServeFileHandler /:id
func ServeFileHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	path, err := GetFile(p.ByName("id"))
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}
	if path == "" {
		w.WriteHeader(404)
		fmt.Fprint(w, "No File for the ID")
		return
	}
	http.ServeFile(w, r, path)
}

// AddFileHandler adds the file path to the database. It should be usually be given to an POST endpoint
// with id as the parameter
// Ex: /file/:id
func AddFileHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	decoder := json.NewDecoder(r.Body)
	// TODO: ps is there for checking emptiness. Should be replaced
	var js, ps jsonStruct
	if err := decoder.Decode(&js); err != nil || js == ps {
		w.WriteHeader(400)
		return
	}

	couchServer, err := couchdb.NewClient("http://127.0.0.1:5984", nil)
	db, _ := couchServer.CreateDB("files")
	userID := memberlist.DefaultWANConfig().Name

	_, err = db.Put(p.ByName("id"), file{UUID: uuid.NewV4().String(), Fname: path.Base(js.Path), UserID: userID}, "")
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}

	// TODO: Send 409 for conflict
	if err := AddFile(p.ByName("id"), js.Path); err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}

	w.WriteHeader(201)
}

// RemoveFileHandler removes the file ref from the db.
// Usually to be used with a DELETE endpoint
// Ex: /file/:id
// BUG: make RESTFUL with custom error type
func RemoveFileHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	couchServer, err := couchdb.NewClient("http://127.0.0.1:5984", nil)
	db, _ := couchServer.CreateDB("files")

	_, err = db.Delete(p.ByName("id"), "")
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}

	err = RemoveFile(p.ByName("id"))
	if err != nil {
		w.WriteHeader(500)
		fmt.Fprint(w, err)
		return
	}
	w.WriteHeader(200)
}

//AddFile Adds file to the db
func AddFile(id string, path string) error {
	db, err := bolt.Open("files.db", 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("files"))
		if err != nil {
			return err
		}

		if b.Get([]byte(id)) == nil {
			// TODO: Add path checking
			return b.Put([]byte(id), []byte(path))
		}
		return errors.New("File already exists for the id")
	})
}

// GetFile gets the path of the file with the id
func GetFile(id string) (string, error) {
	db, err := bolt.Open("files.db", 0600, nil)
	if err != nil {
		return "", err
	}
	defer db.Close()

	var path string

	err = db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("files"))
		if b == nil {
			return errors.New("Bucket doesnt exist")
		}
		path = string(b.Get([]byte(id)))
		return nil
	})

	return path, err
}

// RemoveFile removes the id, path pair from the db
func RemoveFile(id string) error {
	db, err := bolt.Open("files.db", 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("files"))
		if b == nil {
			return errors.New("Bucket doesnt exist")
		}

		if b.Get([]byte(id)) != nil {
			return b.Delete([]byte(id))
		}
		return errors.New("File with ID doesnt exist")
	})
}
