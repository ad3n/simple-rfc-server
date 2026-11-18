#include "bridge.h"

static RFC_RC SAP_API dispatch(RFC_CONNECTION_HANDLE connection, RFC_FUNCTION_HANDLE function, RFC_ERROR_INFO *error) {
    return mockDispatch(connection, function, error);
}

static RFC_RC SAP_API authenticate(RFC_ATTRIBUTES attributes, RFC_AUTHENTICATION_HANDLE handle, RFC_ERROR_INFO *error) {
    return RFC_OK;
}

RFC_RC mockInstall(RFC_FUNCTION_DESC_HANDLE description, RFC_ERROR_INFO *error) {
    return RfcInstallServerFunction(NULL, description, dispatch, error);
}

RFC_RC mockAuth(RFC_ERROR_INFO *error) {
    return RfcInstallAuthenticationCheckHandler(authenticate, error);
}


RFC_RC mockMetadata(RFC_ERROR_INFO *error) {
    RFC_TYPE_DESC_HANDLE row = RfcCreateTypeDesc(cU("RFC_FUNINT"), error);
    if (!row) return error->code;
    const SAP_UC *names[] = {cU("PARAMCLASS"),cU("PARAMETER"),cU("TABNAME"),cU("FIELDNAME"),cU("EXID"),cU("POSITION"),cU("OFFSET"),cU("INTLENGTH"),cU("DECIMALS"),cU("DEFAULT"),cU("PARAMTEXT"),cU("OPTIONAL")};
    unsigned lengths[] = {1,30,30,30,1,4,4,4,4,21,79,1};
    unsigned nuc = 0, uc = 0;
    for (int i = 0; i < 12; i++) {
        RFC_FIELD_DESC field = {0};
        strcpyU(field.name, names[i]);
        field.type = i >= 5 && i <= 8 ? RFCTYPE_INT : RFCTYPE_CHAR;
        field.nucLength = lengths[i];
        field.ucLength = field.type == RFCTYPE_CHAR ? lengths[i]*2 : 4;
        field.nucOffset = nuc; field.ucOffset = uc;
        nuc += field.nucLength; uc += field.ucLength;
        RFC_RC rc = RfcAddTypeField(row, &field, error);
        if (rc != RFC_OK) return rc;
    }
    RFC_RC rc = RfcSetTypeLength(row, (nuc+3)&~3, (uc+3)&~3, error);
    if (rc != RFC_OK) return rc;
    RFC_FUNCTION_DESC_HANDLE desc = RfcCreateFunctionDesc(cU("RFC_GET_FUNCTION_INTERFACE"), error);
    if (!desc) return error->code;
    RFC_PARAMETER_DESC param = {0};
    strcpyU(param.name, cU("FUNCNAME"));
    param.type = RFCTYPE_CHAR; param.direction = RFC_IMPORT; param.nucLength=30; param.ucLength=60;
    rc = RfcAddParameter(desc, &param, error);
    if (rc != RFC_OK) return rc;
    memset(&param, 0, sizeof(param));
    strcpyU(param.name, cU("PARAMS"));
    param.type = RFCTYPE_TABLE; param.direction = RFC_TABLES; param.typeDescHandle = row;
    param.nucLength = (nuc+3)&~3; param.ucLength = (uc+3)&~3;
    rc = RfcAddParameter(desc, &param, error);
    if (rc != RFC_OK) return rc;
    return mockInstall(desc, error);
}
