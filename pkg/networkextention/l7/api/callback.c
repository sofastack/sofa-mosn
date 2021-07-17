#include "api.h"

void runPostCallback(fc f, void *filter, Response resp) {
    f(filter, resp);
    return;
}
