---
# Feel free to add content and custom Front Matter to this file.
# To modify the layout, see https://jekyllrb.com/docs/themes/#overriding-theme-defaults

layout: home
title: Introduction
nav_order: 1
---

# Hello world

Authentication and authorization are essential components of any modern application, and the need for secure and scalable solutions is growing as more and more organizations move to the cloud.

We will introduce a new authentication and authorization microservice that has been designed to be cloud native and run as a sidecar on Kubernetes. This microservice provides a scalable and secure solution for managing user identities and access controls within a cloud-based environment. It offers a range of features and benefits, including support for popular authentication protocols, integration with external identity providers, and the ability to enforce fine-grained access controls.

Unlike traditional monolithic authentication and authorization solutions, our microservice is designed to run as a lightweight sidecar alongside other microservices in a Kubernetes cluster. This allows it to provide robust security without impacting the performance or flexibility of the overall system. Additionally, the use of Kubernetes and other cloud-native technologies allows the microservice to be easily deployed, scaled, and managed in a cloud environment.

In this docs, we will also discuss the key design principles and architecture of the microservice, and explain how it can be integrated with other microservices to provide a complete authentication and authorization solution, and will serve as a guide for those who are interested in using it in their own cloud-based applications.

## Features

- Username & Password Login
- Password reset
- User Sessions
- RBAC Authorization
