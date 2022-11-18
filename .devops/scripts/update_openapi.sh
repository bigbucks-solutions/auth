#! /bin/bash
updateprofilebody=$(<./docs/schemas/update_profile.json);
OLDKEY='paths./user/updateprofile.post.requestBody.content.multipart/form-data.schema.$ref';
KEY='components.schemas.userUpdateProfileBody';
jq --arg k "${KEY}" --arg o "${OLDKEY}" --argjson v "${updateprofilebody}" '. |
setpath($k / "."; $v) | del(.components.schemas."_user_updateprofile_post_request") |
setpath($o / "."; "#/components/schemas/userUpdateProfileBody")' \
./docs/v3/openapi.json  > ./docs/v3/temp.json;
mv ./docs/v3/temp.json ./docs/v3/openapi.json