terraform {
  backend "oci" {
    key = "terraform.tfstate"
  }

  required_providers {
    oci = {
      source  = "oracle/oci"
      version = "~> 7"
    }
  }
}

provider "oci" {
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = var.user_ocid
  fingerprint  = var.fingerprint
  region       = var.region
}
