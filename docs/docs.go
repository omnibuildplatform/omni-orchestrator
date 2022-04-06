// Package docs GENERATED BY THE COMMAND ABOVE; DO NOT EDIT
// This file was generated by swaggo/swag
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/health": {
            "get": {
                "description": "get application health status",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Status"
                ],
                "summary": "Application health",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "$ref": "#/definitions/controller.ApplicationStatus"
                        }
                    }
                }
            }
        },
        "/jobs": {
            "post": {
                "description": "Create a job with specified SPEC",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "tags": [
                    "Job"
                ],
                "summary": "Create Job",
                "parameters": [
                    {
                        "description": "body for create a job",
                        "name": "body",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/application.CreateJobRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "$ref": "#/definitions/common.Job"
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "app.ApplicationInfo": {
            "type": "object",
            "properties": {
                "commitID": {
                    "type": "string"
                },
                "releaseAt": {
                    "type": "string"
                },
                "tag": {
                    "type": "string"
                }
            }
        },
        "application.CreateJobRequest": {
            "type": "object",
            "required": [
                "domain",
                "engine",
                "service",
                "spec",
                "task",
                "userID"
            ],
            "properties": {
                "domain": {
                    "type": "string"
                },
                "engine": {
                    "type": "string"
                },
                "service": {
                    "type": "string"
                },
                "spec": {
                    "type": "object",
                    "additionalProperties": true
                },
                "task": {
                    "type": "string"
                },
                "userID": {
                    "type": "string"
                }
            }
        },
        "common.Job": {
            "type": "object",
            "properties": {
                "detail": {
                    "type": "string"
                },
                "domain": {
                    "type": "string"
                },
                "duration": {
                    "type": "integer"
                },
                "endTime": {
                    "type": "string"
                },
                "engine": {
                    "type": "string"
                },
                "id": {
                    "type": "string"
                },
                "service": {
                    "type": "string"
                },
                "spec": {
                    "type": "object",
                    "additionalProperties": true
                },
                "startTime": {
                    "type": "string"
                },
                "state": {
                    "type": "string"
                },
                "steps": {
                    "type": "array",
                    "items": {
                        "$ref": "#/definitions/common.Step"
                    }
                },
                "task": {
                    "type": "string"
                },
                "userID": {
                    "type": "string"
                }
            }
        },
        "common.Step": {
            "type": "object",
            "properties": {
                "endTime": {
                    "type": "string"
                },
                "id": {
                    "type": "integer"
                },
                "message": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "startTime": {
                    "type": "string"
                },
                "state": {
                    "type": "string"
                }
            }
        },
        "controller.ApplicationStatus": {
            "type": "object",
            "properties": {
                "info": {
                    "$ref": "#/definitions/app.ApplicationInfo"
                },
                "status": {
                    "type": "string"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
	Version:          "",
	Host:             "",
	BasePath:         "",
	Schemes:          []string{},
	Title:            "",
	Description:      "",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
