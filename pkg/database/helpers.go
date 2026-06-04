package database

import (
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReadSaver is the minimal interface for SaveByBusinessKey.
// Both Tx and any lightweight mock satisfy this.
type ReadSaver interface {
	Read() *gorm.DB
	Save(i interface{}) error
}

// ReadCreator is the minimal interface for InsertIfAbsent.
// Both Tx and lightweight tests can satisfy this without exposing all Tx methods.
type ReadCreator interface {
	Read() *gorm.DB
}

// SaveByBusinessKey performs an upsert for composite PK models (TenantID, ID)
// where the non-tenant PK field uses autoIncrement:false and the business
// uniqueness is determined by columns other than the PK.
//
// It queries for an existing record matching businessWhere, copies its ID to
// the model, then delegates to tx.Save(). This ensures the underlying
// ON CONFLICT(tenant_id, id) clause correctly matches the existing row instead
// of inserting a duplicate with a new auto-assigned ID.
//
// The model must be a pointer to a struct with an int "ID" field.
func SaveByBusinessKey(tx ReadSaver, model interface{}, businessWhere string, args ...interface{}) error {
	existing := reflect.New(reflect.TypeOf(model).Elem()).Interface()
	if tx.Read().Where(businessWhere, args...).Select("id").First(existing).Error == nil {
		srcID := reflect.ValueOf(existing).Elem().FieldByName("ID")
		dstID := reflect.ValueOf(model).Elem().FieldByName("ID")
		if srcID.IsValid() && dstID.IsValid() && dstID.CanSet() {
			dstID.Set(srcID)
		}
	}
	return tx.Save(model)
}

// InsertIfAbsent inserts model atomically and ignores conflicts on unique keys.
// It returns true only when a new row was inserted.
func InsertIfAbsent(tx ReadCreator, model interface{}) (bool, error) {
	res := tx.Read().Clauses(clause.OnConflict{DoNothing: true}).Create(model)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
