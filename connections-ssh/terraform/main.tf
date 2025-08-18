resource "oci_core_vcn" "vcn" {
  compartment_id = var.compartment_ocid
  cidr_block     = "10.0.0.0/16"
  dns_label      = "dns"
}

# --- Security rule to allow SSH ---
resource "oci_core_security_list" "ssh" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.vcn.id
  display_name   = "ssh-allow"

  egress_security_rules {
    protocol    = "all"
    destination = "0.0.0.0/0"
  }

  ingress_security_rules {
    protocol = "6" # TCP
    source   = "0.0.0.0/0"
    tcp_options {
      min = 22
      max = 22
    }
  }
}

resource "oci_core_subnet" "subnet" {
  cidr_block     = "10.0.0.0/24"
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.vcn.id
  security_list_ids = [
    oci_core_security_list.ssh.id
  ]
  route_table_id = oci_core_route_table.rt.id
}

resource "oci_core_internet_gateway" "igw" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.vcn.id
  enabled        = true
}

resource "oci_core_route_table" "rt" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.vcn.id

  route_rules {
    network_entity_id = oci_core_internet_gateway.igw.id
    destination       = "0.0.0.0/0"
  }
}

data "oci_identity_availability_domains" "local_ads" {
  compartment_id = var.compartment_ocid
}

# --- Container Instance ---
resource "oci_container_instances_container_instance" "container_instance" {
  compartment_id      = var.compartment_ocid
  availability_domain = data.oci_identity_availability_domains.local_ads.availability_domains[0].name
  display_name        = "tf-connections-ssh"
  shape               = "CI.Standard.A1.Flex"

  shape_config {
    ocpus         = 1
    memory_in_gbs = 1
  }

  vnics {
    subnet_id             = oci_core_subnet.subnet.id
    is_public_ip_assigned = true
  }

  containers {
    image_url    = "lahmanja/connections-ssh:v0.0.11"
    display_name = "connections-ssh"
    command      = ["/connections-ssh", "--port", "22", "--key-file", "id_rsa", "--generate-key"]

    health_checks {
      health_check_type = "TCP"
      port              = 22
    }
  }
}
