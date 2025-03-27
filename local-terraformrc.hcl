provider_installation {
  filesystem_mirror {
    path    = "/tmp/terraform/plugins"
    include = ["*/*"]
  }
  direct {
    exclude = ["*/*"]
  }
}
