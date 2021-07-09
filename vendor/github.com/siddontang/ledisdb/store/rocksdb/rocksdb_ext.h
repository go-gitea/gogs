// +build rocksdb

#ifndef ROCKSDB_EXT_H
#define ROCKSDB_EXT_H

#ifdef __cplusplus
extern "C" {
#endif

#include "rocksdb/c.h"

// Below iterator functions like rocksdb iterator but returns valid status for iterator
extern unsigned char rocksdb_iter_seek_to_first_ext(rocksdb_iterator_t*);
extern unsigned char rocksdb_iter_seek_to_last_ext(rocksdb_iterator_t*);
extern unsigned char rocksdb_iter_seek_ext(rocksdb_iterator_t*, const char* k, size_t klen);
extern unsigned char rocksdb_iter_next_ext(rocksdb_iterator_t*);
extern unsigned char rocksdb_iter_prev_ext(rocksdb_iterator_t*);
extern void rocksdb_write_ext(rocksdb_t* db, const rocksdb_writeoptions_t* options, rocksdb_writebatch_t* batch, char** errptr);

#ifdef __cplusplus
}
#endif

#endif