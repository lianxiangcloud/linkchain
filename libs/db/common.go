package db

import (
	"fmt"
	"github.com/spaolacci/murmur3"
)

const (
	min_db_counts uint64 = 1
	max_db_counts uint64 = 128
)

func dbCountsPreCheck(dbCounts uint64) uint64 {
	if dbCounts < min_db_counts {
		return min_db_counts
	}
	if dbCounts > max_db_counts {
		return max_db_counts
	}
	return dbCounts
}

func dbIndex(key []byte, dbCounts uint64) uint64 {
	if dbCounts == min_db_counts {
		return 0
	}
	return uint64(murmur3.Sum32(key)) % dbCounts
}

func genDbName(name string, index uint64) string {
	if index == 0 {
		return name
	}
	return fmt.Sprintf("%s_%d", name, index)
}

func genStatsKey(key string, index uint64) string {
	return fmt.Sprintf("%s_%d", key, index)
}
