terraform {
  backend "s3" {
    bucket = "ai-events-tfstate-integration"
    key    = "ai-events-streams/rhosak/terraform.tfstate"
  }
}
