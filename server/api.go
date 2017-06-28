package server

import (
    "fmt"
    "net/http"
    "database/sql"
    "encoding/json"

    "github.com/blankbook/shared/models"
    "github.com/blankbook/shared/web"
)

const maxSearchTermLen = 100
const searchTermComponentLength = 3
const numSearchResultsReturned = 20

// SetupAPI adds the API routes to the provided router
func SetupAPI(r web.Router, db *sql.DB) {
    r.HandleRoute([]string{web.GET}, "/groups",
                  []string{"term"},
                  []string{},
                  GetSearchResults, db)
}

func GetSearchResults(w http.ResponseWriter, q map[string]string, b string, db *sql.DB) {
    var err error
    defer func() {
        if err != nil {
            http.Error(w, err.Error(), http.StatusInternalServerError)
        }
    }()

    term := q["term"]
    if len(term) > maxSearchTermLen {
        http.Error(w, fmt.Sprintf("Exceeded max term length of %d", maxSearchTermLen),
                   http.StatusBadRequest)
        return
    }
    if len(term) == 0 {
        http.Error(w, fmt.Sprintf("Cannot search for the empty string"), http.StatusBadRequest)
        return
    }

    // construct a query which, for every group name, calculates the number
    // of subsections of the search term it contains and returns the groups
    // with the highest total
    numTermComponents := (len(term) - 1) / searchTermComponentLength + 1;
    query := `SELECT TOP ($1) `+models.GroupSQLColumns+` FROM (SELECT (`
    const numParamsUsed = 1
    for i := 0; i < numTermComponents; i += 1 {
        if i != 0 {
            query += ` + `
        }
        query += fmt.Sprintf("CASE WHEN Name LIKE $%d THEN 1 ELSE 0 END", i + numParamsUsed + 1)
    }
    query += `) AS numMatches, `+models.GroupSQLColumns+
        ` FROM Groups) xxx WHERE numMatches > 0 ORDER BY numMatches DESC`

    var args []interface{}
    args = append(args, numSearchResultsReturned)
    for i := 0; i < numTermComponents; i += 1 {
        var c string
        if i == numTermComponents - 1 {
            c = term[searchTermComponentLength*i:len(term)]
        } else {
            c = term[searchTermComponentLength*i:searchTermComponentLength*(i+1)]
        }
        args = append(args, "%"+c+"%")
    }

    rows, err := db.Query(query, args...)
    if err != nil { return }
    groups, err := models.GetGroupsFromRows(rows)
    if err != nil { return }
    res, err := json.Marshal(groups)
    if err != nil { return }
    w.Header().Set("Content-Type", "application/json")
    w.Write(res)
}
