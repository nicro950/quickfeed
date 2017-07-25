package web_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/autograde/aguis/models"
	"github.com/autograde/aguis/web"
	"github.com/autograde/aguis/web/auth"
	"github.com/labstack/echo"
)

func TestGetSelf(t *testing.T) {
	const (
		selfURL   = "/user"
		apiPrefix = "/api/v1"
	)

	r := httptest.NewRequest(http.MethodGet, selfURL, nil)
	w := httptest.NewRecorder()
	e := echo.New()
	c := e.NewContext(r, w)

	user := &models.User{ID: 1}
	c.Set(auth.UserKey, user)

	userHandler := web.GetSelf()
	if err := userHandler(c); err != nil {
		t.Error(err)
	}

	userURL := "/users/" + strconv.FormatUint(user.ID, 10)
	location := w.Header().Get("Location")
	if location != apiPrefix+userURL {
		t.Errorf("have Location '%v' want '%v'", location, apiPrefix+userURL)
	}
	assertCode(t, w.Code, http.StatusFound)
}

func TestGetUser(t *testing.T) {
	const (
		getUserRoute = "/users/:uid"

		secret   = "123"
		provider = "github"
		remoteID = 10
	)

	db, cleanup := setup(t)
	defer cleanup()

	user, err := db.CreateUserFromRemoteIdentity(provider, remoteID, secret)
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	router := echo.NewRouter(e)

	// Add the route to handler.
	router.Add(http.MethodGet, getUserRoute, web.GetUser(db))

	userURL := "/users/" + strconv.FormatUint(user.ID, 10)
	r := httptest.NewRequest(http.MethodGet, userURL, nil)
	w := httptest.NewRecorder()
	c := e.NewContext(r, w)
	// Prepare context with user request.
	router.Find(http.MethodGet, userURL, c)

	// Invoke the prepared handler.
	if err := c.Handler()(c); err != nil {
		t.Error(err)
	}

	var foundUser *models.User
	if err := json.Unmarshal(w.Body.Bytes(), &foundUser); err != nil {
		t.Fatal(err)
	}

	// Access token should be stripped.
	user.RemoteIdentities[0].AccessToken = ""
	if !reflect.DeepEqual(foundUser, user) {
		t.Errorf("have user %+v want %+v", foundUser, user)
	}
	assertCode(t, w.Code, http.StatusFound)
}

func TestGetUsers(t *testing.T) {
	const (
		usersURL = "/users"

		remoteID1 = 1
		secret1   = "123"
		provider1 = "github"

		remoteID2 = 2
		secret2   = "456"
		provider2 = "gitlab"
	)

	db, cleanup := setup(t)
	defer cleanup()

	user1, err := db.CreateUserFromRemoteIdentity(provider1, remoteID1, secret1)
	if err != nil {
		t.Fatal(err)
	}
	user2, err := db.CreateUserFromRemoteIdentity(provider2, remoteID2, secret2)
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	r := httptest.NewRequest(http.MethodGet, usersURL, nil)
	w := httptest.NewRecorder()
	c := e.NewContext(r, w)

	h := web.GetUsers(db)
	if err := h(c); err != nil {
		t.Error(err)
	}

	var foundUsers []*models.User
	if err := json.Unmarshal(w.Body.Bytes(), &foundUsers); err != nil {
		t.Fatal(err)
	}

	// Remote identities should not be loaded.
	user1.RemoteIdentities = nil
	user2.RemoteIdentities = nil
	wantUsers := []*models.User{user1, user2}
	if !reflect.DeepEqual(foundUsers, wantUsers) {
		t.Errorf("have users %+v want %+v", foundUsers, wantUsers)
	}

	assertCode(t, w.Code, http.StatusFound)
}

var allUsers = []struct {
	provider string
	remoteID uint64
	secret   string
}{
	{"github", 1, "123"},
	{"github", 2, "456"},
	{"gitlab", 3, "789"},
	{"gitlab", 4, "012"},
	{"bitlab", 5, "345"},
	{"gitlab", 6, "678"},
	{"gitlab", 7, "901"},
	{"gitlab", 8, "234"},
}

func TestGetUsersInCourse(t *testing.T) {
	const (
		usersURL = "/users?course=DAT520"
	)

	db, cleanup := setup(t)
	defer cleanup()

	var users []*models.User
	for _, u := range allUsers {
		user, err := db.CreateUserFromRemoteIdentity(u.provider, u.remoteID, u.secret)
		if err != nil {
			t.Fatal(err)
		}
		// Remote identities should not be loaded.
		user.RemoteIdentities = nil
		users = append(users, user)
	}

	for _, course := range allCourses {
		err := db.CreateCourse(course)
		if err != nil {
			t.Fatal(err)
		}
	}

	// users to enroll in course DAT520 Distributed Systems
	wantUsers := users[0 : len(allUsers)-3]

	// users to enroll in course DAT320 Operating Systems
	osUsers := users[3:7]

	for _, u := range wantUsers {
		enrollment := models.Enrollment{
			UserID:   u.ID,
			CourseID: allCourses[0].ID,
		}
		if err := db.CreateEnrollment(&enrollment); err != nil {
			t.Fatal(err)
		}
		if err := db.AcceptEnrollment(enrollment.ID); err != nil {
			t.Fatal(err)
		}
	}

	for _, u := range osUsers {
		enrollment := models.Enrollment{
			UserID:   u.ID,
			CourseID: allCourses[1].ID,
		}
		if err := db.CreateEnrollment(&enrollment); err != nil {
			t.Fatal(err)
		}
		if err := db.AcceptEnrollment(enrollment.ID); err != nil {
			t.Fatal(err)
		}
	}

	e := echo.New()
	r := httptest.NewRequest(http.MethodGet, usersURL, nil)
	w := httptest.NewRecorder()
	c := e.NewContext(r, w)

	h := web.GetUsers(db)
	if err := h(c); err != nil {
		t.Error(err)
	}

	var foundUsers []*models.User
	if err := json.Unmarshal(w.Body.Bytes(), &foundUsers); err != nil {
		t.Fatal(err)
	}
	for _, u := range foundUsers {
		// Remote identities should not be loaded.
		u.RemoteIdentities = nil
	}

	if !reflect.DeepEqual(foundUsers, wantUsers) {
		t.Errorf("have users %+v want %+v", foundUsers, wantUsers)
	}

	assertCode(t, w.Code, http.StatusFound)
}

func TestPatchUser(t *testing.T) {
	const (
		getUserRoute = "/users/:uid"

		secret   = "123"
		provider = "github"
		remoteID = 10
	)

	db, cleanup := setup(t)
	defer cleanup()

	user, err := db.CreateUserFromRemoteIdentity(provider, remoteID, secret)
	if err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	router := echo.NewRouter(e)

	// Add the route to handler.
	router.Add(http.MethodPatch, getUserRoute, web.PatchUser(db))

	// Send empty request, the user should not be modified.
	emptyRequest := web.UpdateUserRequest{}
	emptyJSON, err := json.Marshal(&emptyRequest)
	if err != nil {
		t.Fatal(err)
	}

	userURL := "/users/" + strconv.FormatUint(user.ID, 10)
	r := httptest.NewRequest(http.MethodPatch, userURL, bytes.NewBuffer(emptyJSON))
	r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	w := httptest.NewRecorder()
	c := e.NewContext(r, w)
	// Prepare context with user request.
	router.Find(http.MethodPatch, userURL, c)

	// Invoke the prepared handler.
	if err := c.Handler()(c); err != nil {
		t.Error(err)
	}
	assertCode(t, w.Code, http.StatusNotModified)

	tmp := true
	// Send request with IsAdmin set to true, the user should become admin.
	trueRequest := web.UpdateUserRequest{IsAdmin: &tmp}
	trueJSON, err := json.Marshal(&trueRequest)
	if err != nil {
		t.Fatal(err)
	}

	r = httptest.NewRequest(http.MethodPatch, userURL, bytes.NewBuffer(trueJSON))
	r.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	w = httptest.NewRecorder()
	c = e.NewContext(r, w)
	// Prepare context with user request.
	router.Find(http.MethodPatch, userURL, c)

	// Invoke the prepared handler.
	if err := c.Handler()(c); err != nil {
		t.Error(err)
	}
	assertCode(t, w.Code, http.StatusOK)

	admin, err := db.GetUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}

	if !admin.IsAdmin {
		t.Error("expected user to have become admin")
	}
}