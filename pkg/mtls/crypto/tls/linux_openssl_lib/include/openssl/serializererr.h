/*
 * Generated by util/mkerr.pl DO NOT EDIT
 * Copyright 1995-2019 The OpenSSL Project Authors. All Rights Reserved.
 *
 * Licensed under the Apache License 2.0 (the "License").  You may not use
 * this file except in compliance with the License.  You can obtain a copy
 * in the file LICENSE in the source distribution or at
 * https://www.openssl.org/source/license.html
 */

#ifndef OPENSSL_OSSL_SERIALIZERERR_H
# define OPENSSL_OSSL_SERIALIZERERR_H

# include <openssl/opensslconf.h>
# include <openssl/symhacks.h>


# ifdef  __cplusplus
extern "C"
# endif
int ERR_load_OSSL_SERIALIZER_strings(void);

/*
 * OSSL_SERIALIZER function codes.
 */
# ifndef OPENSSL_NO_DEPRECATED_3_0
# endif

/*
 * OSSL_SERIALIZER reason codes.
 */
# define OSSL_SERIALIZER_R_INCORRECT_PROPERTY_QUERY       100

#endif
