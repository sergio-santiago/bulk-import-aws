# infra/bootstrap

Bootstrap one-off para `bulk-import-aws`. Crea los recursos previos al stack
principal: bucket S3 para el state remoto de Terraform y tabla DynamoDB de
locking.

Usa state **local** (el bucket aún no existe cuando se aplica). El fichero
`terraform.tfstate` queda en este directorio, ignorado por git. Conviene
guardarlo aparte si quieres poder destruir el bootstrap más adelante.

## Restricciones del entorno

Este proyecto se despliega contra una cuenta AWS de tipo lab con permisos IAM
muy limitados (no se pueden crear roles, users, OIDC providers ni access keys).
Por eso el bootstrap **no** crea un rol para GitHub Actions ni configuración
OIDC. El despliegue se opera localmente con la sesión SSO; la CI/CD se limita
a validación de Terraform y linting (ver README raíz para detalles).

## Pre-requisitos

- Terraform >= 1.6.
- AWS CLI con perfil SSO activo apuntando a la cuenta destino.

## Aplicación

```bash
export AWS_PROFILE=lab-sergio

cd infra/bootstrap
terraform init
terraform plan
terraform apply
```

## Destrucción

```bash
terraform destroy
```

Si el bucket de state tiene objetos (state del stack principal), vaciarlo
manualmente antes. Sin objetos, `destroy` borra todo limpiamente.
