health_checks {
    listener_enabled = true
    bind_address = "0.0.0.0"
    bind_port = "8081"
    live_path = "/live"
    ready_path = "/ready"
}
plugins {
    NodeAttestor "join_token" {
        plugin_data {}
    }
}
  
