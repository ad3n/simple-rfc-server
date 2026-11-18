#include <sapnwrfc.h>
#include <stdlib.h>
extern RFC_RC mockDispatch(RFC_CONNECTION_HANDLE, RFC_FUNCTION_HANDLE, RFC_ERROR_INFO*);
RFC_RC mockInstall(RFC_FUNCTION_DESC_HANDLE, RFC_ERROR_INFO*);
RFC_RC mockAuth(RFC_ERROR_INFO*);

RFC_RC mockMetadata(RFC_ERROR_INFO*);
