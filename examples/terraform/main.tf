locals {
  data = [
    for k, v in var.service_accounts : {
      service_account = google_service_account.sa[k].email
      bucket          = google_storage_bucket.sa_keys[k].name
      days            = v.days
      renewal_window  = v.renewal_window
    }
  ]

  flattened_iam_members = flatten([
    for k, v in var.service_accounts : [
      for u in v.users : {
        account = k
        bucket  = google_storage_bucket.sa_keys[k].name
        member  = u
      }
    ]
  ])

  iam_members = {
    for m in local.flattened_iam_members : "${m.account}/${m.member}" => m
  }
}

resource "google_service_account" "key_rotator" {
  project      = var.project
  account_id   = "key-rotator"
  display_name = "Key Rotator service account"
}

resource "google_cloud_run_service" "key_rotator" {
  name     = "key-rotator"
  project  = var.project
  location = "europe-west1"
  template {
    spec {
      service_account_name = google_service_account.key_rotator.email
      containers {
        image = var.image
        args  = ["server"]
      }
    }
  }
  traffic {
    percent         = 100
    latest_revision = true
  }
}

resource "google_cloud_run_service_iam_member" "member" {
  location = google_cloud_run_service.key_rotator.location
  service  = google_cloud_run_service.key_rotator.name
  role     = "roles/run.invoker"
  member   = "serviceAccount:${google_service_account.key_rotator.email}"
}

resource "google_cloud_scheduler_job" "job" {
  name             = "key-rotator"
  project          = var.project
  region           = var.location
  schedule         = "0 4 * * *"
  time_zone        = var.time_zone
  attempt_deadline = "320s"

  http_target {
    http_method = "POST"
    uri         = google_cloud_run_service.key_rotator.status[0].url
    body        = base64encode(jsonencode(local.data))

    oidc_token {
      service_account_email = google_service_account.key_rotator.email
    }
  }
}


// === 'managed' service accounts

resource "google_service_account" "sa" {
  for_each   = var.service_accounts
  project    = var.project
  account_id = each.key
}

resource "google_storage_bucket" "sa_keys" {
  for_each      = var.service_accounts
  name          = "${each.key}-keys"
  project       = var.project
  location      = var.location
  force_destroy = true

  uniform_bucket_level_access = true
}

resource "google_service_account_iam_member" "sa_rotator_iam" {
  for_each           = var.service_accounts
  service_account_id = google_service_account.sa[each.key].name
  role               = "roles/iam.serviceAccountKeyAdmin"
  member             = "serviceAccount:${google_service_account.key_rotator.email}"
}

resource "google_storage_bucket_iam_member" "sa_rotator_iam" {
  for_each = var.service_accounts
  bucket   = google_storage_bucket.sa_keys[each.key].name
  role     = "roles/storage.admin"
  member   = "serviceAccount:${google_service_account.key_rotator.email}"
}

resource "google_storage_bucket_iam_member" "users" {
  for_each = local.iam_members
  bucket   = each.value.bucket
  role     = "roles/storage.objectViewer"
  member   = each.value.member
}