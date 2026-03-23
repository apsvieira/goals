terraform {
  required_version = ">= 1.0"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

locals {
  project_id = "gym-tracking-app-456023"
  region     = "southamerica-east1"
}

provider "google" {
  project = local.project_id
  region  = local.region
}

resource "google_project_service" "play_developer_api" {
  service            = "androidpublisher.googleapis.com"
  disable_on_destroy = false
}

resource "google_service_account" "play_publisher" {
  account_id   = "play-publisher"
  display_name = "Play Store Publisher"
  description  = "Service account for automated Play Store uploads via Gradle Play Publisher"
}

resource "google_service_account_key" "play_publisher_key" {
  service_account_id = google_service_account.play_publisher.name
}

resource "local_file" "play_publisher_key_json" {
  content  = base64decode(google_service_account_key.play_publisher_key.private_key)
  filename = "${path.module}/../play-publisher-key.json"

  file_permission = "0600"
}

output "service_account_email" {
  value       = google_service_account.play_publisher.email
  description = "Grant this email permissions in Play Console > Setup > API access"
}
