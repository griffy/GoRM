package gorm

import (
	"errors"
	"fmt"
	"strings"
	"reflect"
	"exp/sql"
)

type columnValue struct {
	Val interface{}
}

func (c *columnValue) ScanInto(value interface{}) error {
	// TODO: handle types better
	switch v := value.(type) {
	case []byte:
		// since v is volatile, we must create a copy
		c.Val = string(v)
	default:
		c.Val = v
	}
	return nil
}

type Conn struct {
	DB *sql.DB
}

func (c *Conn) Close() error {
	return c.DB.Close()
}

func NewConnection(driverName, dataSource string) (*Conn, error) {
	db, err := sql.Open(driverName, dataSource)
	return &Conn{db}, err
}

type Session struct {
	*Conn
	*sql.Tx	
}

func (c *Conn) NewSession() (*Session, error) {
	tx, err := c.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Session{c, tx}, nil
}

// This should only be called after a Commit or Rollback
func (s *Session) Renew() error {
	// since a transaction's life ends after a commit
	// or rollback, we must create a new one
	tx, err := s.Conn.DB.Begin()
	if err != nil {
		return err
	}
	s.Tx = tx
	return nil
}

func getTableName(obj interface{}) string {
	return pluralizeString(snakeCasedName(getTypeName(obj)))
}

func (s *Session) getResultsForQuery(tableName string, args ...interface{}) (results []map[string]interface{}, err error) {
	queryArgs := []interface{}{}
	condition := ""
	if len(args) >= 1 {
		condition, _ = args[0].(string)
		if len(args) > 1 {
			queryArgs = args[1:]
		}
	}
	query := fmt.Sprintf("select * from %v %v", 
						 tableName, 
						 condition)
	rows, err := s.Conn.DB.Query(query, queryArgs...)
	if err != nil {
		return nil, err
	}
	columnNames, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	numColumns := len(columnNames)
	for rows.Next() {
		// create a slice the size of the number of
		// columns
		row := make([]*columnValue, numColumns)
		for i := 0; i < numColumns; i++ {
			row[i] = &columnValue{nil}
		}
		// store them in a slice of interfaces to send to scan
		rowAsInterfaces := make([]interface{}, numColumns)
		for i, cv := range row {
			rowAsInterfaces[i] = cv
		}
		
		// populate the row slice
		err := rows.Scan(rowAsInterfaces...) // ...
		if err != nil {
			return nil, err
		}

		// a result has the column name as the key
		// and column val as the value
		result := make(map[string]interface{})
		// store each column in the map
		for columnNum, val := range rowAsInterfaces {
			cv := val.(*columnValue)
			// store a copy of the val in the result map
			columnName := columnNames[columnNum]
			result[columnName] = cv.Val
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *Session) insert(tableName string, properties map[string]interface{}) (int64, error) {
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

func (s *Session) Update(rowStruct interface{}) error {
	results, _ := scanStructIntoMap(rowStruct)
	tableName := getTableName(rowStruct)

	id := results["id"].(int64)
	delete(results, "id")

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
	_, err := s.Tx.Exec(stmt, args...)
	return err
}

func (s *Session) Save(rowStruct interface{}) error {
	err := s.Update(rowStruct)
	if err != nil {
		return err
	}
	return s.Tx.Commit()
}

func (s *Session) Get(rowStruct interface{}, args ...interface{}) error {
	conditionStr := ""
	switch args[0].(type) {
	case int:
		// add the condition string to the beginning of the args
		// slice so the int comes second
		conditionStr = "id = ?"
		ci := reflect.ValueOf(conditionStr).Interface()
		args = append([]interface{}{}, 
					  append([]interface{}{ci}, args...)...)
	}

	// modify the condition so it's part of a where clause
	args[0] = reflect.ValueOf(fmt.Sprintf("where %v", args[0])).Interface()
	resultsSlice, err := s.getResultsForQuery(getTableName(rowStruct), args...)
	if err != nil {
		return err
	}

	switch len(resultsSlice) {
	case 0:
		return errors.New("did not find any results")
	case 1:
		results := resultsSlice[0]
		scanMapIntoStruct(rowStruct, results)
	default:
		return errors.New("more than one row matched")
	}

	return nil
}

// TODO: test
func (s *Session) GetAll(rowsSlicePtr interface{}, args ...interface{}) error {
	sliceValue := reflect.Indirect(reflect.ValueOf(rowsSlicePtr))
	if sliceValue.Kind() != reflect.Slice {
		return errors.New("needs a pointer to a slice")
	}

	sliceElementType := sliceValue.Type().Elem()

	queryArgs := []interface{}{}
	condition := ""
	if len(args) >= 1 {
		condition = strings.TrimSpace(args[0].(string))
		condition = fmt.Sprintf("where %v", condition)
		if len(args) > 1 {
			queryArgs = args[1:]
		}
	}

	resultsSlice, err := s.getResultsForQuery(getTableName(rowsSlicePtr), condition, queryArgs)
	if err != nil {
		return err
	}

	for _, results := range resultsSlice {
		newValue := reflect.Zero(sliceElementType)
		//println("newValue = ", sliceElementType.String())
		scanMapIntoStruct(newValue.Addr().Interface(), results)
		sliceValue.Set(reflect.Append(sliceValue, newValue))
	}

	return nil
}
