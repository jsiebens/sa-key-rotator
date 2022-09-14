variable "project" {
  type = string
}

variable "location" {
  type = string
}

variable "time_zone" {
  type = string
}

variable "image" {
  type = string
}

variable "service_accounts" {
  type = map(object({
    days           = number
    renewal_window = number
    users          = list(string)
  }))
}