// +build rocksdb

#include "rocksdb_ext.h"

#include <stdlib.h>
#include <string>

extern "C" {

unsigned char rocksdb_iter_seek_to_first_ext(rocksdb_iterator_t* iter) {
    rocksdb_iter_seek_to_first(iter);
    return rocksdb_iter_valid(iter);
}

unsigned char rocksdb_iter_seek_to_last_ext(rocksdb_iterator_t* iter) {
    rocksdb_iter_seek_to_last(iter);
    return rocksdb_iter_valid(iter);
}

unsigned char rocksdb_iter_seek_ext(rocksdb_iterator_t* iter, const char* k, size_t klen) {
    rocksdb_iter_seek(iter, k, klen);
    return rocksdb_iter_valid(iter);
}

unsigned char rocksdb_iter_next_ext(rocksdb_iterator_t* iter) {
    rocksdb_iter_next(iter);
    return rocksdb_iter_valid(iter);
}

unsigned char rocksdb_iter_prev_ext(rocksdb_iterator_t* iter) {
    rocksdb_iter_prev(iter);
    return rocksdb_iter_valid(iter);
}

void rocksdb_write_ext(rocksdb_t* db, 
    const rocksdb_writeoptions_t* options, 
    rocksdb_writebatch_t* batch, char** errptr) {
    rocksdb_write(db, options, batch, errptr);
    if(*errptr == NULL) {
        rocksdb_writebatch_clear(batch);
    }
}

}