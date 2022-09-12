---
layout: home
title: Command Line
nav_order: 2
---

# Create a user role

The command to create a user role.

```bash
    auth role create <role_name>
```

# Create a permission

Create a permission object which can be binded to any role, This binded permissions are checked against role during authorization.

```bash
    auth add-permission <PERMISSION_CODE> --description <PERMISSION_DESCRIPTION> --resource <PERMISSION_RESOURCE>
```

# Bind permission to role

Bind the specified permission to the role

```bash
auth role bind-permission ROLE_NAME PERM_NAME
```