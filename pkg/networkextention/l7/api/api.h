#pragma once

#include <stdint.h>

// cgo request struct
typedef struct {
    void *filter;
    const char **headers;
    const char **trailers;
    const char *req_body[2];
    uint64_t cid;
    uint64_t sid;
}Request;

// cgo response struct
typedef struct {
    const char **headers;
    const char **trailers;
    char *resp_body[2];
    int   status;
    int   need_async;
    int   direct_response;
}Response;

// post callback function type
typedef void (*fc)(void *filter, Response resp);

extern void runPostCallback(fc f, void *filter, Response resp);
