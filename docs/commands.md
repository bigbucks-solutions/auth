---
layout: home
title: Command Line
nav_order: 2
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
    auth add-permission <PERMISSION_NAME> --description <PERMISSION_DESCRIPTION> --resource <PERMISSION_RESOURCE>
    # eg: auth add-permission VIEW-ACC-REPORT --description "Allows to view accounts report" --resource ACCOUNTS 
```

# Bind permission to role

Bind the specified permission to the role

```bash
auth role bind-permission <ROLE_NAME> <PERMISSION_NAME>
# eg: auth role bind-permission ADMIN VIEW-ACC-REPORT

```