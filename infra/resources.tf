resource "digitalocean_droplet" "confbot" {
  image = "${var.image}"
  name = "confbot"
  region = "${var.region}"
  size = "${var.app_size}"
  private_networking = true
  ssh_keys = [
    "${var.ssh_fingerprint}"
  ]
  user_data = "${template_file.user_data.rendered}"

  connection {
    user = "${var.user}"
    type = "ssh"
    key_file = "${var.private_key}"
    timeout = "2m"
  }
}

resource "digitalocean_record" "confbot" {
  domain = "${var.domain}"
  type = "A"
  name = "${digitalocean_droplet.confbot.name}"
  value = "${digitalocean_droplet.confbot.ipv4_address}"
}

resource "template_file" "user_data" {
  template = "${file("${path.module}/conf/cloud-config.yaml")}"

  vars {
    public_key = "${var.public_key}"
    user = "${var.user}"

  }
}