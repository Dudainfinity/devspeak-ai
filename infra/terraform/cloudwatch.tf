###############################################################################
# CloudWatch — alarmes básicos pra EC2 do DevSpeak.
#
# Ativação:
#   Setar var.alert_email pra ativar notificações por email (SNS).
#   Sem email setado, os alarmes existem mas não notificam (ainda visíveis no
#   console + CloudWatch).
#
#   $ cd infra/terraform
#   $ terraform apply -var alert_email=voce@exemplo.com
#
# Custo: alarmes são $0.10/mês cada. SNS email é gratuito. Métricas detalhadas
# (1 min) custariam $2.10/mês — aqui usamos as padrão (5 min) que são gratuitas.
###############################################################################

variable "alert_email" {
  description = "Email para receber alertas dos alarmes CloudWatch. Vazio = sem notificação."
  type        = string
  default     = ""
}

# ── SNS topic + subscription opcional ──────────────────────────────────────────

resource "aws_sns_topic" "devspeak_alerts" {
  name = "devspeak-alerts"
}

resource "aws_sns_topic_subscription" "email_alert" {
  count     = var.alert_email == "" ? 0 : 1
  topic_arn = aws_sns_topic.devspeak_alerts.arn
  protocol  = "email"
  endpoint  = var.alert_email
}

# ── Alarmes ────────────────────────────────────────────────────────────────────

resource "aws_cloudwatch_metric_alarm" "high_cpu" {
  alarm_name          = "devspeak-ec2-high-cpu"
  alarm_description   = "CPU > 80% por 10 minutos seguidos"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "CPUUtilization"
  namespace           = "AWS/EC2"
  period              = 300
  statistic           = "Average"
  threshold           = 80

  dimensions = {
    InstanceId = aws_instance.devspeak_server.id
  }

  alarm_actions = [aws_sns_topic.devspeak_alerts.arn]
  ok_actions    = [aws_sns_topic.devspeak_alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "instance_status_failed" {
  alarm_name          = "devspeak-ec2-status-check-failed"
  alarm_description   = "EC2 status check falhou (instance unreachable)"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "StatusCheckFailed_Instance"
  namespace           = "AWS/EC2"
  period              = 60
  statistic           = "Maximum"
  threshold           = 0

  dimensions = {
    InstanceId = aws_instance.devspeak_server.id
  }

  alarm_actions = [aws_sns_topic.devspeak_alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "system_status_failed" {
  alarm_name          = "devspeak-ec2-system-status-failed"
  alarm_description   = "EC2 system status check falhou (problema na infra AWS)"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "StatusCheckFailed_System"
  namespace           = "AWS/EC2"
  period              = 60
  statistic           = "Maximum"
  threshold           = 0

  dimensions = {
    InstanceId = aws_instance.devspeak_server.id
  }

  alarm_actions = [aws_sns_topic.devspeak_alerts.arn]
}

resource "aws_cloudwatch_metric_alarm" "network_anomaly" {
  alarm_name          = "devspeak-ec2-traffic-spike"
  alarm_description   = "Tráfego de saída > 100MB em 5 min (possível abuso/scraping)"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "NetworkOut"
  namespace           = "AWS/EC2"
  period              = 300
  statistic           = "Sum"
  threshold           = 100 * 1024 * 1024 # bytes

  dimensions = {
    InstanceId = aws_instance.devspeak_server.id
  }

  alarm_actions = [aws_sns_topic.devspeak_alerts.arn]
}

output "alerts_sns_topic" {
  value       = aws_sns_topic.devspeak_alerts.arn
  description = "ARN do SNS topic usado pelos alarmes. Confirme o email no inbox antes que comece a entregar."
}
