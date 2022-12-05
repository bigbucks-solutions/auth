// Package docs GENERATED BY SWAG; DO NOT EDIT
// This file was generated by swaggo/swag
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "termsOfService": "http://swagger.io/terms/",
        "contact": {
            "name": "Jamsheed",
            "url": "http://www.swagger.io/support",
            "email": "jamsheed@nsmail.dev"
        },
        "license": {
            "name": "Apache 2.0",
            "url": "http://www.apache.org/licenses/LICENSE-2.0.html"
        },
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/me": {
            "get": {
                "security": [
                    {
                        "JWTAuth": []
                    }
                ],
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Get logged in user profile information",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Authorization",
                        "name": "X-Auth",
                        "in": "header",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/types.UserInfo"
                        }
                    },
                    "400": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        },
        "/signin": {
            "post": {
                "description": "Authenticate user with password and issue jwt token",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Authenticate with username and pssword",
                "parameters": [
                    {
                        "description": "request body",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/controllers.JsonCred"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "string"
                        }
                    },
                    "400": {
                        "description": ""
                    },
                    "404": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        },
        "/user/authorize": {
            "post": {
                "security": [
                    {
                        "JWTAuth": []
                    }
                ],
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Check user have permission",
                "parameters": [
                    {
                        "description": "request body",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/types.CheckPermissionBody"
                        }
                    },
                    {
                        "type": "string",
                        "description": "Authorization",
                        "name": "X-Auth",
                        "in": "header",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/types.AuthorizeResponse"
                        }
                    },
                    "400": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        },
        "/user/changepassword/{token}": {
            "post": {
                "description": "Reset the password with the password reset token sent to email",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Reset the password with the password reset token sent",
                "parameters": [
                    {
                        "type": "string",
                        "description": "token",
                        "name": "token",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "request body",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/controllers.ResetPassword"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "return"
                    },
                    "400": {
                        "description": ""
                    },
                    "404": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        },
        "/user/reset": {
            "post": {
                "description": "Get password reset token to email",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Send the password reset token",
                "parameters": [
                    {
                        "description": "request body",
                        "name": "request",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/controllers.RequestPasswordResetToken"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "return",
                        "schema": {
                            "$ref": "#/definitions/types.SimpleResponse"
                        }
                    },
                    "400": {
                        "description": ""
                    },
                    "404": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        },
        "/user/updateprofile": {
            "post": {
                "security": [
                    {
                        "JWTAuth": []
                    }
                ],
                "description": "Update user profile details",
                "consumes": [
                    "multipart/form-data"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "auth"
                ],
                "summary": "Update User profile details",
                "parameters": [
                    {
                        "type": "string",
                        "description": "Authorization",
                        "name": "X-Auth",
                        "in": "header",
                        "required": true
                    },
                    {
                        "type": "file",
                        "description": "formData",
                        "name": "request",
                        "in": "formData",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": ""
                    },
                    "400": {
                        "description": ""
                    },
                    "404": {
                        "description": ""
                    },
                    "500": {
                        "description": ""
                    }
                }
            }
        }
    },
    "definitions": {
        "bigbucks_solution_auth_rest-api_controllers_types.Profile": {
            "type": "object",
            "properties": {
                "avatar": {
                    "type": "string"
                },
                "email": {
                    "type": "string"
                },
                "firstName": {
                    "type": "string"
                },
                "lastName": {
                    "type": "string"
                },
                "phone": {
                    "type": "string"
                }
            }
        },
        "bigbucks_solution_auth_rest-api_controllers_types.Role": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string"
                }
            }
        },
        "controllers.JsonCred": {
            "type": "object",
            "properties": {
                "password": {
                    "type": "string"
                },
                "recaptcha": {
                    "type": "string"
                },
                "username": {
                    "type": "string"
                }
            }
        },
        "controllers.RequestPasswordResetToken": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string",
                    "example": "example@example.com"
                }
            }
        },
        "controllers.ResetPassword": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string"
                },
                "password": {
                    "type": "string"
                }
            }
        },
        "types.AuthorizeResponse": {
            "type": "object",
            "properties": {
                "status": {
                    "type": "boolean"
                }
            }
        },
        "types.CheckPermissionBody": {
            "type": "object",
            "properties": {
                "orgID": {
                    "type": "integer"
                },
                "permission": {
                    "type": "string"
                },
                "resource": {
                    "type": "string"
                }
            }
        },
        "types.SimpleResponse": {
            "type": "object",
            "properties": {
                "message": {
                    "type": "string",
                    "example": "message"
                }
            }
        },
        "types.UserInfo": {
            "type": "object",
            "properties": {
                "isSocialAccount": {
                    "type": "boolean"
                },
                "profile": {
                    "$ref": "#/definitions/bigbucks_solution_auth_rest-api_controllers_types.Profile"
                },
                "roles": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/bigbucks_solution_auth_rest-api_controllers_types.Role"
                    }
                },
                "username": {
                    "type": "string"
                }
            }
        }
    },
    "securityDefinitions": {
        "JWTAuth": {
            "type": "apiKey",
            "name": "X-Auth",
            "in": "header"
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "0.0.1",
	Host:             "localhost:8000",
	BasePath:         "/api/v1",
	Schemes:          []string{},
	Title:            "BigBucks Solutions Auth Engine",
	Description:      "This is REST api definitions.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
