# bulk-import-aws

Importación masiva asíncrona de productos en AWS. Arquitectura serverless
event-driven (S3 + Lambda + SQS + DynamoDB) desplegada con Terraform.
Proyecto final de curso AWS.

## Arquitectura

Diagrama y resumen del flujo de una importación, desde la subida del fichero
hasta el report agregado, con los puntos donde intervienen S3, SQS, las tres
Lambdas (`api`, `parser`, `worker`) y DynamoDB.

## Decisión arquitectónica

Justificación corta del stack elegido frente a las alternativas razonables
(serverless event-driven vs. contenedores, DynamoDB vs. RDS, SQS vs. otras
opciones de mensajería), con foco en coste, operabilidad y ajuste al MVP.

## Deploy

Pasos reproducibles desde cero: bootstrap manual del backend de Terraform,
configuración de credenciales y región, `terraform apply` por capas, y
publicación del frontend estático.

## Destroy

Cómo derribar todo el entorno (Terraform destroy del stack principal y, si
procede, limpieza manual del bootstrap) sin dejar recursos huérfanos que
generen coste.

## Coste estimado

Aproximación honesta del coste mensual con el entorno arriba sin tráfico
real, asumiendo free tier vigente para la cuenta.
