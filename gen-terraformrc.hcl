provider_installation {
  filesystem_mirror {
    path    = "./.terraform/plugins"
    include = ["*/*"]
  }
  direct {
    exclude = ["*/*"]
  }
}
