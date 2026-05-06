server {
    bind_address = "0.0.0.0"
    bind_port = 8080
    trust_domain = "example.com"
    data_dir = "/work/spidar/server"
    log_level = "info"
    socket_path = "/work/spidar/server/server.sock"
}
health_checks {
    listener_enabled = true
    bind_address = "0.0.0.0"
    bind_port = "8081"
    live_path = "/live"
    ready_path = "/ready"
}
plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "/work/spidar/server/datastore.sqlite3"
        }
    }
    KeyManager "disk" {
        plugin_data {
            keys_path = "/work/spidar/server/keys.json"
        }
    }
    NodeAttestor "join_token" {
        plugin_data {}
    }

}
workloads {
  server "hello" {
    data = "ganga"
  }
}
  
