variable "project" {}
variable "domain" {}
variable "public_key" {}

variable "image" {
  default = "dokku"
}

variable "region" {}

variable "user" {
  default = "ubuntu"
}

variable "app_size" {
  default = "4gb"
}
