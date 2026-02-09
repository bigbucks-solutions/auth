---
layout: home
title: Command Line
nav_order: 3
---

# Create a user role

The command to create a user role.

```bash
    auth role create <ROLE_NAME>
    # eg: auth role create ADMIN
```

# Create a permission

Create a permission object which can be binded to any role, This binded permissions are checked against role during authorization.

```bash
    auth create-permission --description <PERMISSION_DESCRIPTION> --resource <PERMISSION_RESOURCE> --scope <SCOPE> --action <ACTION>
    # eg: auth create-permission --description "all account permission" --resource "accounts" --scope "org" --action "read"
```

# Bind permission to role

Bind the specified permission to the role

```bash
auth role bind-permission <ROLE_ID> --resource <PERMISSION_RESOURCE> --scope <SCOPE> --action <ACTION> --orgid <ORG_ID>
# eg: auth role bind-permission ADMIN VIEW-ACC-REPORT

```
