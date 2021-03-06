package mysql

import (
	"github.com/galaxy-book/common/core/logger"
	"github.com/galaxy-book/common/core/util/strs"
	"upper.io/db.v3/lib/sqlbuilder"
)

func Insert(obj Domain) error {
	conn, err := GetConnect()
	defer func() {
		if conn != nil {
			if err := conn.Close(); err != nil {
				logger.GetDefaultLogger().Info(strs.ObjectToString(err))
			}
		}
	}()
	if err != nil {
		return err
	}
	_, err = conn.Collection(obj.TableName()).Insert(obj)
	if err != nil {
		return err
	}
	return nil
}

func InsertReturnId(obj Domain) (interface{}, error) {
	conn, err := GetConnect()
	defer func() {
		if conn != nil {
			if err := conn.Close(); err != nil {
				logger.GetDefaultLogger().Info(strs.ObjectToString(err))
			}
		}
	}()
	if err != nil {
		return nil, err
	}
	id, err := conn.Collection(obj.TableName()).Insert(obj)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func TransInsert(tx sqlbuilder.Tx, obj Domain) error {
	_, err := tx.Collection(obj.TableName()).Insert(obj)
	if err != nil {
		return err
	}
	return nil
}

func TransInsertReturnId(tx sqlbuilder.Tx, obj Domain) (interface{}, error) {
	id, err := tx.Collection(obj.TableName()).Insert(obj)
	if err != nil {
		return nil, err
	}
	return id, nil
}

func TransBatchInsert(tx sqlbuilder.Tx, obj Domain, objs []interface{}) error {

	//a := objs.([]interface{})

	batch := tx.InsertInto(obj.TableName()).Batch(len(objs))
	go func() {
		defer batch.Done()
		for i := range objs {
			batch.Values(objs[i])
		}
	}()
	err := batch.Wait()
	if err != nil {
		return err
	}

	return nil
}

func BatchInsert(obj Domain, objs []interface{}) error {
	conn, err := GetConnect()
	defer func() {
		if conn != nil {
			if err := conn.Close(); err != nil {
				logger.GetDefaultLogger().Info(strs.ObjectToString(err))
			}
		}
	}()
	if err != nil {
		log.Error(strs.ObjectToString(err))
		return err
	}

	batch := conn.InsertInto(obj.TableName()).Batch(len(objs))
	go func() {
		defer batch.Done()
		for i := range objs {
			batch.Values(objs[i])
		}
	}()
	err = batch.Wait()
	if err != nil {
		return err
	}

	return nil
}

//func BatchDone(pos []interface{}, batch *sqlbuilder.BatchInserter) {
//		defer batch.Done()
//		for i := range pos {
//			batch.Values(pos[i])
//		}
//}
