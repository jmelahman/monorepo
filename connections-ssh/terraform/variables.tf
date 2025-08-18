variable "tenancy_ocid" {
  type        = string
  description = "The OCID of the tenancy"
}

variable "compartment_ocid" {
  type        = string
  description = "The OCID of the compartment"
}

variable "user_ocid" {
  type        = string
  description = "The OCID of the user"
}

variable "fingerprint" {
  type        = string
  description = "The fingerprint of the API key"
}

variable "region" {
  type        = string
  description = "The region where resources will be created"
}
