package gorm

import (
	"os"
	"fmt"
	"strings"
	"reflect"
	"exp/sql"
)

type Conn struct {
	conn *sql.DB
}

func (c *Conn) Close() os.Error {
	return c.conn.Close()
}

func NewConnection(driverName, dataSourceName string) (*Conn, os.Error) {
	conn, err := sql.Open(driverName, dataSourceName)
	return &Conn{conn: conn}, err
}

type Session struct {
	*Conn
	*sql.Tx	
}

func (c *Conn) NewSession() *Session, os.Error {
	tx, err := c.Begin()
	if err != nil {
		return nil, err
	}
	return &Session{c, tx}, nil
}

// This should only be called after a Commit or Rollback
func (s *Session) Renew() os.Error {
	// since a session has the lifetime of a transaction,
	// we must create a new transaction and therefore 
	// new session and assign it back
	s, err = s.Conn.NewSession()
	if err != nil {
		return err
	}
	return nil
}

func getTableName(obj interface{}) string {
	return pluralizeString(snakeCasedName(getTypeName(obj)))
}

func (s *Session) getResultsForQuery(tableName, args ...interface{}) (results []map[string][]byte, err os.Error) {
	queryArgs := []interface{}
	condition := ""
	if len(args) >= 1 {
		condition = args[0].type(string)
		if len(args) > 1 {
			queryArgs = args[1:]
		}
	}
	query := fmt.Sprintf("select * from %v %v", 
						 tableName, 
						 condition)
	stmt, err := s.Tx.Prepare(query)
	if err != nil {
		return nil, err
	}

	rows, err := stmt.Query(queryArgs)
	if err != nil {
		return nil, err
	}

	columnNames, err := s.conn.Columns()
	if err != nil {
		return nil, err
	}

	numColumns := len(columnNames)
	for rows.Next() {
		// create an array the size of the number of
		// columns, where each column is a slice of bytes
		// and return a pointer (slice) to that
		row := &new([numColumns][]byte)
		// populate the row array
		err := rows.Scan(row...)
		if err != nil {
			return nil, err
		}
		// a result has the column name as the key
		// and column val as the value
		result := make(map[string][]byte)
		// since the slices filled by scan are not values,
		// they won't be valid after this iteration. So,
		// we must make copies of the data that are
		for column, _ := range row {
			val := new([len(row[column])]byte)
			for i, b := range row[column] {
				val[i] = b
			}
			// store a slice of the fresh copy
			// in our result map
			columnName := columnNames[column]
			result[columnName] = &val
		}
		results = append(results, result)
	}
	return results, nil
}

func (s *Session) insert(tableName string, properties map[string]interface{}) (int64, os.Error) {
	var keys []string
	var placeholders []string
	var args []interface{}

	for key, val := range properties {
		keys = append(keys, key)
		placeholders = append(placeholders, "?")
		args = append(args, val)
	}

	stmt := fmt.Sprintf("insert into %v (%v) values (%v)",
						 tableName,
						 strings.Join(keys, ", "),
						 strings.Join(placeholders, ", "))

	res, err := s.Tx.Exec(stmt, args...)
	if err != nil {
		return -1, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return -1, err
	}
	return id, nil
}

func (s *Session) Update(rowStruct interface{}) os.Error {
	results, _ := scanStructIntoMap(rowStruct)
	tableName := getTableName(rowStruct)

	id := results["id"]
	results["id"] = 0, false

	if id == 0 {
		id, err := s.insert(tableName, results)
		if err != nil {
			return nil
		}

		structPtr := reflect.ValueOf(rowStruct)
		structVal := structPtr.Elem()
		structField := structVal.FieldByName("Id")
		structField.Set(reflect.ValueOf(id))

		return nil
	}

	var updates []string
	var args []interface{}

	for key, val := range results {
		updates = append(updates, fmt.Sprintf("%v = ?", key))
		args = append(args, val)
	}

	stmt := fmt.Sprintf("update %v set %v where id = %v",
						 tableName,
						 strings.Join(updates, ", "),
						 id)

	return s.Tx.Exec(stmt, args...)
}

func (s *Session) Save(rowStruct interface{}) os.Error {
	err := s.Update(rowStruct)
	if err != nil {
		return err
	}
	return s.Tx.Commit()
}

func (s *Session) Get(rowStruct interface{}, condition interface{}, args ...interface{}) os.Error {
	conditionStr := ""

	switch condition := condition.(type) {
	case string:
		conditionStr = condition
	case int:
		conditionStr = "id = ?"
		args = append(args, condition)
	}

	conditionStr = fmt.Sprintf("where %v", conditionStr)

	resultsSlice, err := s.getResultsForQuery(getTableName(rowStruct), conditionStr, args)
	if err != nil {
		return err
	}

	switch len(resultsSlice) {
	case 0:
		return os.NewError("did not find any results")
	case 1:
		results := resultsSlice[0]
		scanMapIntoStruct(rowStruct, results)
	default:
		return os.NewError("more than one row matched")
	}

	return nil
}

func (s *Session) GetAll(rowsSlicePtr interface{}, args ...interface{}) os.Error {
	sliceValue := reflect.Indirect(reflect.ValueOf(rowsSlicePtr))
	if sliceValue.Kind() != reflect.Slice {
		return os.NewError("needs a pointer to a slice")
	}

	sliceElementType := sliceValue.Type().Elem()

	queryArgs := []interface{}
	condition := ""
	if len(args) >= 1 {
		condition = strings.TrimSpace(args[0].type(string))
		condition = fmt.Sprintf("where %v", condition)
		if len(args) > 1 {
			queryArgs = args[1:]
		}
	}

	resultsSlice, err := c.getResultsForQuery(getTableName(rowsSlicePtr), condition, queryArgs)
	if err != nil {
		return err
	}
	/*
	var a int; println(reflect.ValueOf(a).CanAddr())
	println(reflect.Zero(reflect.TypeOf(a)).CanAddr())
	println(reflect.Zero(reflect.TypeOf(42)).Addr().String())
	*/
	for _, results := range resultsSlice {
		newValue := reflect.Zero(sliceElementType)
		//println("newValue = ", sliceElementType.String())
		scanMapIntoStruct(newValue.Addr().Interface(), results)
		sliceValue.Set(reflect.Append(sliceValue, newValue))
	}

	return nil
}
