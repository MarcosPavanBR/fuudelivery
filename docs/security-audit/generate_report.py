#!/usr/bin/env python3
"""
FuuDelivery — Security Audit Report Generator
Generates a professional PDF report with charts, findings, and GitHub issues.
"""

import os
import io
from datetime import datetime
from reportlab.lib.pagesizes import A4
from reportlab.lib.units import cm, mm
from reportlab.lib.colors import HexColor, white, black
from reportlab.lib.styles import getSampleStyleSheet, ParagraphStyle
from reportlab.lib.enums import TA_CENTER, TA_LEFT, TA_JUSTIFY, TA_RIGHT
from reportlab.platypus import (
    SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle,
    PageBreak, Image, KeepTogether
)
from reportlab.graphics.shapes import Drawing, String, Circle, Wedge, Line
from reportlab.graphics.charts.piecharts import Pie
from reportlab.graphics.charts.barcharts import VerticalBarChart
from reportlab.graphics import renderPDF

# ── Colors ──────────────────────────────────────────────────────────
CRITICAL = HexColor("#B91C1C")
HIGH = HexColor("#EA580C")
MEDIUM = HexColor("#D97706")
LOW = HexColor("#2563EB")
INFO = HexColor("#6B7280")
STRONG = HexColor("#059669")
BG_LIGHT = HexColor("#F9FAFB")
BORDER = HexColor("#E5E7EB")
HEADER_BG = HexColor("#1F2937")

# ── Styles ──────────────────────────────────────────────────────────
styles = getSampleStyleSheet()

style_title = ParagraphStyle('Title2', parent=styles['Title'], fontSize=28, textColor=HEADER_BG, spaceAfter=6, fontName='Helvetica-Bold')
style_subtitle = ParagraphStyle('Subtitle2', parent=styles['Normal'], fontSize=14, textColor=HexColor("#6B7280"), spaceAfter=20, fontName='Helvetica')
style_h1 = ParagraphStyle('H1', parent=styles['Heading1'], fontSize=20, textColor=HEADER_BG, spaceBefore=24, spaceAfter=12, fontName='Helvetica-Bold')
style_h2 = ParagraphStyle('H2', parent=styles['Heading2'], fontSize=16, textColor=HexColor("#374151"), spaceBefore=16, spaceAfter=8, fontName='Helvetica-Bold')
style_h3 = ParagraphStyle('H3', parent=styles['Heading3'], fontSize=12, textColor=HexColor("#4B5563"), spaceBefore=12, spaceAfter=6, fontName='Helvetica-Bold')
style_body = ParagraphStyle('Body2', parent=styles['Normal'], fontSize=10, textColor=HexColor("#1F2937"), spaceAfter=8, leading=14, alignment=TA_JUSTIFY, fontName='Helvetica')
style_code = ParagraphStyle('Code2', parent=styles['Normal'], fontSize=8, textColor=HexColor("#1F2937"), spaceAfter=6, leading=10, fontName='Courier', backColor=HexColor("#F3F4F6"), borderPadding=4)
style_bullet = ParagraphStyle('Bullet2', parent=styles['Normal'], fontSize=10, textColor=HexColor("#1F2937"), spaceAfter=4, leading=13, leftIndent=16, bulletIndent=0, fontName='Helvetica')
style_small = ParagraphStyle('Small', parent=styles['Normal'], fontSize=8, textColor=HexColor("#9CA3AF"), fontName='Helvetica')
style_footer = ParagraphStyle('Footer', parent=styles['Normal'], fontSize=8, textColor=HexColor("#9CA3AF"), alignment=TA_CENTER, fontName='Helvetica')

# ── Findings Data ───────────────────────────────────────────────────
findings = [
    # Category 1: Database without tenant isolation
    {
        "cat": "Banco sem tranca",
        "sev": "Crítica",
        "file": "Backend/auth_api/app/handlers/user_handler.go",
        "line": "22-32",
        "title": "GetUser sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode ler os dados completos de QUALQUER outro usuário (nome, email, telefone, role, establishment_id) simplesmente passando o ID na URL.",
        "code": 'userID := c.Params("id")\nvar user models.User\nmodels.DB.First(&user, userID)\nreturn c.JSON(user)  // retorna TUDO, sem checagem',
        "exploit": "IDOR clássico: GET /users/1, GET /users/2, etc. Um restaurante pode descobrir dados de outros restaurantes ou clientes.",
    },
    {
        "cat": "Banco sem tranca",
        "sev": "Crítica",
        "file": "Backend/orders_api/app/handlers/orders.go",
        "line": "363-385",
        "title": "ListOrdersByEstablishmentID sem isolamento",
        "desc": "Qualquer usuário autenticado pode listar todos os pedidos de qualquer estabelecimento, incluindo dados de clientes, valores, telefones.",
        "code": 'establishmentID := c.Params("establishmentId")\nmodels.DB.Where("establishment_id = ?", establishmentIDInt).\n    Order("created_at desc").Limit(500).Find(&docs)\n// Sem canActOnEstablishment()!',
        "exploit": "GET /orders/1 (estabelecimento do dono) funciona — mas GET /orders/2 (estabelecimento concorrente) também funciona. Vazamento massivo de dados.",
    },
    {
        "cat": "Banco sem tranca",
        "sev": "Alta",
        "file": "Backend/orders_api/app/handlers/coupon.go",
        "line": "6-18",
        "title": "ListCoupons sem isolamento de estabelecimento",
        "desc": "Lista cupons sem filtrar pelo establishment_id do token. O parâmetro é query param, não do token.",
        "code": 'func ListCoupons(c *fiber.Ctx) error {\n    establishmentID := c.Query("establishment_id")\n    query := models.DB\n    if establishmentID != "" {\n        query = query.Where("establishment_id = ? OR establishment_id = 0", establishmentID)\n    }\n    query.Find(&coupons) // qualquer establishment_id funciona',
        "exploit": "GET /coupons?establishment_id=1 — qualquer autenticado pode listar cupons de qualquer restaurante.",
    },
    {
        "cat": "Banco sem tranca",
        "sev": "Alta",
        "file": "Backend/auth_api/app/handlers/establishment_handler.go",
        "line": "6-20",
        "title": "GetUserByEstablishment sem verificação de posse",
        "desc": "Lista todos os usuários (nome, email, id) de qualquer estabelecimento sem verificar se o chamador pertence ao estabelecimento.",
        "code": 'func GetUserByEstablishment(c *fiber.Ctx) error {\n    establishmentId, _ := c.ParamsInt("id")\n    models.DB.Where(&models.User{EstablishmentID: uint(establishmentId)}).Find(&user)\n    return c.JSON(user) // lista TODOS os users do estabelecimento',
        "exploit": "GET /establishments/1/users — qualquer autenticado pode listar todos os funcionários de qualquer restaurante.",
    },
    # Category 2: Permission only in browser
    {
        "cat": "Permissão só no navegador",
        "sev": "Média",
        "file": "Frontend/WebAdmin/src/App.js",
        "line": "47-57",
        "title": "WebAdmin ProtectedRoute não verifica role admin",
        "desc": "O componente ProtectedRoute do WebAdmin apenas checa se o usuário está logado (user != null), mas NÃO verifica se é admin. Qualquer role (restaurant, client, delivery) pode acessar o WebAdmin.",
        "code": "const ProtectedRoute = ({ children }) => {\n  const { user, loading } = useAuth();\n  if (!user) return <Navigate to=\"/login\" replace />;\n  return children; // sem checagem de role!\n};",
        "exploit": "Um restaurante pode acessar WebAdmin > Gerenciamento de Usuários, ESTABELECIMENTOS, etc. O backend bloqueia com adminRequired, mas a UI fica visível.",
    },
    # Category 3: IDOR
    {
        "cat": "IDOR",
        "sev": "Crítica",
        "file": "Backend/orders_api/app/handlers/coupon.go",
        "line": "24-42",
        "title": "DeleteCoupon sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode desativar cupons de qualquer estabelecimento.",
        "code": 'func DeleteCoupon(c *fiber.Ctx) error {\n    id := c.Params("id")\n    models.DB.First(&coupon, id)\n    coupon.IsActive = false\n    models.DB.Save(&coupon) // sem checar se pertence ao establishment do token',
        "exploit": "DELETE /coupons/999 — restaurante A pode desativar cupons do restaurante B.",
    },
    {
        "cat": "IDOR",
        "sev": "Alta",
        "file": "Backend/payment_api/app/handlers/order_total.go",
        "line": "6-22",
        "title": "GetPaymentByOrder sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode consultar detalhes de pagamento de qualquer pedido.",
        "code": 'func GetPaymentByOrder(c *fiber.Ctx) error {\n    orderID := c.Params("order_id")\n    models.DB.Where("order_id = ?", orderID).First(&payment)\n    return c.JSON(payment) // sem checar ownership',
        "exploit": "GET /payments/order/abc123 — qualquer autenticado pode ver valor, método e status de pagamento de qualquer pedido.",
    },
    {
        "cat": "IDOR",
        "sev": "Alta",
        "file": "Backend/auth_api/app/handlers/establishment_handler.go",
        "line": "28-40",
        "title": "HandlerEstablishmentStatus sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode abrir/fechar qualquer restaurante, alterando o OpenData.",
        "code": 'func HandlerEstablishmentStatus(c *fiber.Ctx) error {\n    establishmentID := c.Params("id")\n    models.DB.First(&establishment, establishmentID)\n    // toggle OpenData sem canActOnEstablishment()',
        "exploit": "PUT /establishments/status/handler/2 — restaurante A pode fechar o restaurante B.",
    },
    {
        "cat": "IDOR",
        "sev": "Alta",
        "file": "Backend/auth_api/app/handlers/business_hours.go",
        "line": "6-35",
        "title": "UpsertBusinessHours sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode alterar horários de funcionamento de qualquer restaurante passando establishment_id no body.",
        "code": 'func UpsertBusinessHours(c *fiber.Ctx) error {\n    hours := models.BusinessHours{\n        EstablishmentID: req.EstablishmentID, // do BODY, não do token!\n        DayOfWeek: req.DayOfWeek,\n        IsOpen: req.IsOpen,\n    }\n    // sem canActOnEstablishment(req.EstablishmentID)',
        "exploit": "POST /establishments/hours com establishment_id=2 — restaurante A altera horários do restaurante B.",
    },
    {
        "cat": "IDOR",
        "sev": "Alta",
        "file": "Backend/orders_api/app/handlers/coupon.go",
        "line": "20-23",
        "title": "GetCoupon sem verificação de posse",
        "desc": "Qualquer usuário autenticado pode ler detalhes de cupons de qualquer restaurante.",
        "code": 'func GetCoupon(c *fiber.Ctx) error {\n    id := c.Params("id")\n    models.DB.First(&coupon, id)\n    return c.JSON(coupon) // sem ownership check',
        "exploit": "GET /coupons/5 — qualquer autenticado pode ver código, desconto e regras de cupons alheios.",
    },
    # Category 4: Exposed keys
    {
        "cat": "Chaves expostas",
        "sev": "Baixa",
        "file": "Backend/auth_api/app/handlers/*.go",
        "line": "múltiplos",
        "title": "Senha mínima de 6 caracteres",
        "desc": "Todas as rotas de criação de conta e reset de senha aceitam senhas de apenas 6 caracteres, abaixo do mínimo recomendado (8+).",
        "code": 'if len(req.Password) < 6 {\n    return c.Status(400).JSON(fiber.Map{"error": "..."})\n}',
        "exploit": "Contas com senhas fracas como '123456' ou 'abcdef' são vulneráveis a brute force.",
    },
    # Category 5: XSS
    {
        "cat": "XSS",
        "sev": "Baixa",
        "file": "Frontend/WebRestaurant/src/components/CardapioList.js",
        "line": "30",
        "title": "src={item?.Image} renderiza URL do usuário sem validação",
        "desc": "A URL da imagem do cardapio e renderizada diretamente no atributo src sem validacao de protocolo. Embora browsers modernos bloqueiem javascript: em src, URLs como data: com HTML embutido podem causar problemas.",
        "code": '&lt;img src={item?.Image} /&gt;',
        "exploit": "Se o backend não validar o protocolo da URL, um atacante pode enviar data:text/html,... como URL de imagem.",
    },
    {
        "cat": "XSS",
        "sev": "Baixa",
        "file": "Frontend/WebRestaurant/src/",
        "line": "global",
        "title": "Sem biblioteca de sanitização (DOMPurify ou similar)",
        "desc": "O frontend não usa nenhuma biblioteca de sanitização de HTML. Embora não haja dangerouslySetInnerHTML, a ausência de sanitização é um risco se o projeto crescer e começar a renderizar HTML do servidor.",
        "code": "// Nenhum import de DOMPurify, xss, ou sanitize-html encontrado",
        "exploit": "Risco latent: se qualquer componente futuro usar dangerouslySetInnerHTML, não há barreira de segurança.",
    },
]

# Strengths (verified correct)
strengths = [
    ("Gateway Router com fallback", "pkg/gateway/router.go", "Router.Select() implementa fallback automático Pagar.me → Asaas → AbacatePay com circuit breaker"),
    ("Carteira com SELECT FOR UPDATE", "Backend/payment_api/app/handlers/wallet.go", "Operações financeiras usam transação + FOR UPDATE para prevenir race conditions"),
    ("Webhook HMAC fail-closed", "Backend/payment_api/app/handlers/webhook.go", "Sem ABACATE_PAY_WEBHOOK_SECRET em produção → rejeita (antes só logava)"),
    ("CSRF Protection", "cmd/fuudelivery/main.go:1530-1540", "Middleware valida X-CSRF-Token em todas as mutações POST/PUT/DELETE"),
    ("Rate Limiting", "cmd/fuudelivery/main.go", "Login 10/min, refresh 30/min, mutations protegidas com rate limiter"),
    ("Upload de imagem validado", "cmd/fuudelivery/pkg/upload/upload.go", "Magic bytes + bloqueio SVG + ownership check por entidade"),
    ("IDOR em UpdateProduct", "Backend/orders_api/app/handlers/products.go", "canActOnEstablishment() antes de permitir alteração"),
    ("IDOR em DeleteProduct", "Backend/orders_api/app/handlers/products.go", "canActOnEstablishment() antes de permitir exclusão"),
    ("IDOR em UpdateOrderStatus", "Backend/orders_api/app/handlers/orders.go:246", "canActOnEstablishment(doc.EstablishmentID) antes de mudar status"),
    ("WebSocket com ticket", "cmd/fuudelivery/main.go:806-870", "resolveWSTicket() + wsCanAccessOrder() antes de conexão"),
    ("Startup validation", "cmd/fuudelivery/main.go:1410-1440", "Fatality se JWT_SECRET ou DB_CONNECTION_STRING ausentes em produção"),
    ("Sessão HttpOnly", "Backend/auth_api/app/handlers/session_handler.go", "Cookies com HttpOnly + Secure + SameSite=None"),
    ("EstablishmentWithdraw com ownership", "Backend/payment_api/app/handlers/wallet.go", "GetEstablishmentIDFromToken() + ownership check"),
    ("TopUp/Deduct com ownership", "Backend/payment_api/app/handlers/wallet.go", "tokenUserID != req.UserID → rejeita"),
]

# ── Severity counts ─────────────────────────────────────────────────
sev_counts = {"Crítica": 0, "Alta": 0, "Média": 0, "Baixa": 0}
cat_counts = {}
for f in findings:
    sev_counts[f["sev"]] = sev_counts.get(f["sev"], 0) + 1
    cat_counts[f["cat"]] = cat_counts.get(f["cat"], 0) + 1

# ── PDF Generation ──────────────────────────────────────────────────
def build_pdf(output_path):
    doc = SimpleDocTemplate(
        output_path,
        pagesize=A4,
        leftMargin=2*cm, rightMargin=2*cm,
        topMargin=2.5*cm, bottomMargin=2.5*cm
    )

    story = []
    W = A4[0] - 4*cm  # available width

    # ── Helper: colored chip ────────────────────────────────────────
    def sev_chip(sev):
        color_map = {"Crítica": CRITICAL, "Alta": HIGH, "Média": MEDIUM, "Baixa": LOW, "Informativa": INFO, "Forte": STRONG}
        c = color_map.get(sev, INFO)
        return f'<font color="#FFFFFF" backColor="{c.hexval()}">&nbsp;<b>{sev}</b>&nbsp;</font>'

    # ── Cover page ──────────────────────────────────────────────────
    story.append(Spacer(1, 6*cm))
    story.append(Paragraph("Relatório de Auditoria de Segurança", style_title))
    story.append(Spacer(1, 0.5*cm))
    story.append(Paragraph("FuuDelivery", ParagraphStyle('Proj', parent=style_title, fontSize=22, textColor=HexColor("#DC2626"))))
    story.append(Spacer(1, 1*cm))
    story.append(Paragraph(f"Data: {datetime.now().strftime('%d/%m/%Y')}", style_subtitle))
    story.append(Paragraph("Escopo: Backend Go (Fiber/GORM), Frontends React (WebRestaurant, WebAdmin), Deploy Render", style_subtitle))
    story.append(Spacer(1, 1*cm))
    story.append(Paragraph(
        "<b>Metodologia:</b> Cada categoria de segurança foi mapeada para a stack detectada — "
        "isolamento de dados via queries GORM com filtro manual por establishment_id/user_id "
        "(ausência de RLS no Supabase, projeto usa GORM direto); permissões via middleware Go "
        "(adminRequired, protectedRoute, canActOnEstablishment); IDOR via varredura sistemática "
        "de handlers com parâmetros de ID; chaves via grep de hardcoded values; XSS via análise "
        "de JSX e sanitize libraries.",
        ParagraphStyle('Meta', parent=style_body, fontSize=9, textColor=HexColor("#6B7280"))
    ))
    story.append(PageBreak())

    # ── Executive Summary ───────────────────────────────────────────
    story.append(Paragraph("1. Resumo Executivo", style_h1))
    story.append(Paragraph(
        f"A auditoria identificou <b>{len(findings)} achados</b> de segurança: "
        f"<font color='{CRITICAL.hexval()}'><b>{sev_counts['Crítica']}</b> críticos</font>, "
        f"<font color='{HIGH.hexval()}'><b>{sev_counts['Alta']}</b> altos</font>, "
        f"<font color='{MEDIUM.hexval()}'><b>{sev_counts['Média']}</b> médios</font>, "
        f"<font color='{LOW.hexval()}'><b>{sev_counts['Baixa']}</b> baixos</font>. "
        f"<b>{len(strengths)} pontos fortes</b> verificados.",
        style_body
    ))
    story.append(Spacer(1, 0.5*cm))

    # ── Pie chart ───────────────────────────────────────────────────
    pie_drawing = Drawing(250, 150)
    pie = Pie()
    pie.x = 60
    pie.y = 10
    pie.width = 120
    pie.height = 120
    pie.data = [sev_counts["Crítica"], sev_counts["Alta"], sev_counts["Média"], sev_counts["Baixa"]]
    pie.labels = [f'Crítica ({sev_counts["Crítica"]})', f'Alta ({sev_counts["Alta"]})', f'Média ({sev_counts["Média"]})', f'Baixa ({sev_counts["Baixa"]})']
    pie.slices.strokeWidth = 0.5
    pie.slices.strokeColor = white
    pie.slices[0].fillColor = CRITICAL
    pie.slices[1].fillColor = HIGH
    pie.slices[2].fillColor = MEDIUM
    pie.slices[3].fillColor = LOW
    pie.sideLabels = True
    pie.simpleLabels = False
    pie.slices.fontName = 'Helvetica'
    pie.slices.fontSize = 8
    pie_drawing.add(pie)
    story.append(pie_drawing)
    story.append(Spacer(1, 0.5*cm))

    # ── Bar chart by category ───────────────────────────────────────
    bar_drawing = Drawing(450, 160)
    bc = VerticalBarChart()
    bc.x = 60
    bc.y = 30
    bc.height = 110
    bc.width = 360
    bc.data = [[cat_counts.get(c, 0) for c in ["Banco sem tranca", "Permissão só no navegador", "IDOR", "Chaves expostas", "XSS"]]]
    bc.categoryAxis.categoryNames = ["Banco s/ tranca", "Perm. só browser", "IDOR", "Chaves expostas", "XSS"]
    bc.categoryAxis.labels.fontName = 'Helvetica'
    bc.categoryAxis.labels.fontSize = 8
    bc.categoryAxis.labels.angle = 0
    bc.valueAxis.valueMin = 0
    bc.valueAxis.valueMax = max(cat_counts.values()) + 1
    bc.valueAxis.valueStep = 1
    bc.bars[0].fillColor = HIGH
    bc.bars[0].strokeColor = None
    bar_drawing.add(bc)
    story.append(bar_drawing)
    story.append(Spacer(1, 0.5*cm))

    # ── Strengths ───────────────────────────────────────────────────
    story.append(Paragraph("2. Pontos Fortes Verificados", style_h1))
    for title, file, desc in strengths:
        story.append(Paragraph(
            f'<font color="{STRONG.hexval()}">✅</font> <b>{title}</b> — <font size="8" color="#6B7280">{file}</font>',
            style_bullet
        ))
        story.append(Paragraph(f'&nbsp;&nbsp;&nbsp;&nbsp;{desc}', ParagraphStyle('SubBullet', parent=style_small, leftIndent=30)))
    story.append(PageBreak())

    # ── Detailed Findings Table ─────────────────────────────────────
    story.append(Paragraph("3. Achados Detalhados", style_h1))

    # Group by category
    categories_order = ["Banco sem tranca", "Permissão só no navegador", "IDOR", "Chaves expostas", "XSS"]
    finding_num = 0
    for cat in categories_order:
        cat_findings = [f for f in findings if f["cat"] == cat]
        if not cat_findings:
            continue
        story.append(Paragraph(f"3.{categories_order.index(cat)+1} {cat}", style_h2))

        for f in cat_findings:
            finding_num += 1
            sev_c = {"Crítica": CRITICAL, "Alta": HIGH, "Média": MEDIUM, "Baixa": LOW}.get(f["sev"], INFO)

            # Finding header
            story.append(Paragraph(
                f'<font color="{sev_c.hexval()}"><b>[{f["sev"]}]</b></font> '
                f'<b>{f["title"]}</b>',
                ParagraphStyle('FindTitle', parent=style_body, fontSize=11, spaceBefore=8, spaceAfter=2, fontName='Helvetica-Bold')
            ))
            story.append(Paragraph(
                f'<font size="8" color="#6B7280">{f["file"]}:{f["line"]}</font>',
                style_small
            ))
            story.append(Paragraph(f["desc"], style_body))

            # Code block
            code_escaped = f["code"].replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;")
            story.append(Paragraph(
                f'<font face="Courier" size="7">{code_escaped}</font>',
                style_code
            ))

            # Exploit
            story.append(Paragraph(
                f'<b>Exploração:</b> {f["exploit"]}',
                ParagraphStyle('Exploit', parent=style_body, fontSize=9, textColor=HexColor("#7C2D12"), backColor=HexColor("#FEF2F2"), borderPadding=4)
            ))
            story.append(Spacer(1, 0.3*cm))

    story.append(PageBreak())

    # ── Recommendations ─────────────────────────────────────────────
    story.append(Paragraph("4. Recomendações Priorizadas", style_h1))

    recs = [
        ("P1", "Corrigir IDOR em GetUser, ListOrdersByEstablishmentID, GetCoupon, DeleteCoupon, GetPaymentByOrder, GetUserByEstablishment, HandlerEstablishmentStatus, UpsertBusinessHours, BulkUpdateBusinessHours — adicionar canActOnEstablishment() ou equivalente em TODOS os handlers que acessam dados por ID"),
        ("P1", "Adicionar ownership check em ListCoupons e ListCoupons filtrar automaticamente pelo establishment_id do token, não por query param"),
        ("P2", "WebAdmin ProtectedRoute deve verificar role === 'admin' antes de renderizar children"),
        ("P2", "Aumentar senha mínima para 8 caracteres + complexidade (maiúscula, número, especial)"),
        ("P3", "Validar protocolo de URLs de imagem (permitir apenas https:// e data:image/) no frontend"),
        ("P3", "Adicionar DOMPurify ou equivalente para sanitização defensiva no frontend"),
    ]

    for priority, text in recs:
        p_color = {"P1": CRITICAL, "P2": MEDIUM, "P3": LOW}.get(priority, INFO)
        story.append(Paragraph(
            f'<font color="{p_color.hexval()}"><b>[{priority}]</b></font> {text}',
            style_bullet
        ))
    story.append(Spacer(1, 1*cm))

    # ── GitHub Issues ───────────────────────────────────────────────
    story.append(Paragraph("5. Issues para o GitHub", style_h1))

    # Issue 1: IDOR batch
    issue1 = """--- ISSUE 1 ---
**[Segurança] IDOR em múltiplos handlers — dados de outros estabelecimentos acessíveis**

**Labels:** `security` `priority:critical`

**Descrição:**
Múltiplos handlers não verificam se o objeto pertence ao establishment_id do usuário autenticado. Qualquer usuário comum pode ler, alterar ou deletar recursos de outros restaurantes.

**Evidência:**
- `Backend/auth_api/app/handlers/user_handler.go:22-32` — `GetUser` retorna dados completos de qualquer usuário por ID
- `Backend/orders_api/app/handlers/orders.go:363-385` — `ListOrdersByEstablishmentID` lista pedidos de qualquer estabelecimento
- `Backend/orders_api/app/handlers/coupon.go:6-42` — `ListCoupons`, `GetCoupon`, `DeleteCoupon` sem ownership
- `Backend/payment_api/app/handlers/order_total.go:6-22` — `GetPaymentByOrder` expõe dados financeiros
- `Backend/auth_api/app/handlers/establishment_handler.go:6-20` — `GetUserByEstablishment` lista funcionários
- `Backend/auth_api/app/handlers/establishment_handler.go:28-40` — `HandlerEstablishmentStatus` altera status
- `Backend/auth_api/app/handlers/business_hours.go:6-35` — `UpsertBusinessHours` altera horários

**Impacto:**
Vazamento de dados pessoais, manipulação de pedidos, cupons e configurações de restaurantes concorrentes.

**Sugestão de correção:**
Adicionar `canActOnEstablishment(c, establishmentID)` ou equivalente em TODOS os handlers que acessam dados por ID de estabelecimento. Para GetUser, restringir a: próprio perfil ou admin.

**Critérios de aceite:**
- [ ] Todos os handlers de leitura por ID verificam posse
- [ ] Todos os handlers de escrita verificam posse
- [ ] Testes unitários provam que usuários não podem acessar dados alheios
--- FIM ISSUE 1 ---"""

    story.append(Paragraph(issue1.replace("\n", "<br/>").replace("&", "&amp;"), style_code))
    story.append(Spacer(1, 0.5*cm))

    # Issue 2: WebAdmin role check
    issue2 = """--- ISSUE 2 ---
**[Segurança] WebAdmin não verifica role admin no frontend**

**Labels:** `security` `priority:medium`

**Descrição:**
O componente `ProtectedRoute` em `Frontend/WebAdmin/src/App.js:47-57` apenas verifica se o usuário está logado, sem checar se é admin. Qualquer role pode acessar o WebAdmin.

**Evidência:**
```javascript
const ProtectedRoute = ({ children }) => {
  const { user, loading } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  return children; // sem checagem de role!
};
```

**Impacto:**
Interface de admin visível para restaurantes/clientes/entregadores, mesmo que o backend bloqueie as chamadas API.

**Sugestão de correção:**
Adicionar `if (user?.role !== 'admin') return <Navigate to="/" replace />;` no ProtectedRoute.

**Critérios de aceite:**
- [ ] WebAdmin ProtectedRoute verifica `user.role === 'admin'`
- [ ] Usuários não-admin são redirecionados para página apropriada
--- FIM ISSUE 2 ---"""

    story.append(Paragraph(issue2.replace("\n", "<br/>").replace("&", "&amp;"), style_code))
    story.append(Spacer(1, 0.5*cm))

    # Issue 3: Password strength
    issue3 = """--- ISSUE 3 ---
**[Segurança] Senha mínima de 6 caracteres é fraca**

**Labels:** `security` `priority:low`

**Descrição:**
Todas as rotas de criação de conta e reset de senha aceitam senhas de apenas 6 caracteres, abaixo do mínimo NIST (8+).

**Evidência:**
`Backend/auth_api/app/handlers/user_handler.go:329`, `client_handler.go:35`, `deliveryman_handler.go:146`, `establishment_handler.go:39`, `password_reset.go:24`

**Impacto:**
Contas com senhas fracas são vulneráveis a brute force e credential stuffing.

**Sugestão de correção:**
Aumentar mínimo para 8 caracteres + exigir pelo menos 1 maiúscula, 1 número.

**Critérios de aceite:**
- [ ] Senha mínima >= 8 caracteres
- [ ] Exigência de complexidade documentada na UI
--- FIM ISSUE 3 ---"""

    story.append(Paragraph(issue3.replace("\n", "<br/>").replace("&", "&amp;"), style_code))

    # ── Footer ──────────────────────────────────────────────────────
    story.append(PageBreak())
    story.append(Paragraph("Anexo — Stack Detectada", style_h1))
    story.append(Paragraph("""
    <b>Backend:</b> Go 1.25, Fiber v2.52, GORM v1.31 (PostgreSQL via Supabase), golang-jwt/v5 (HS256), Redis Streams<br/>
    <b>Frontend:</b> React 19, Axios, Vite, React Router v6, react-toastify<br/>
    <b>Mobile:</b> React Native + Expo (3 apps: AppComida, AppEntrega, AppRestaurante)<br/>
    <b>Deploy:</b> Render (render.yaml), Dockerfile existente<br/>
    <b>CI/CD:</b> GitHub Actions (15+ jobs)<br/>
    <b>Database:</b> PostgreSQL (Supabase), Redis externo (*.db.redis.io)<br/>
    <b>Auth:</b> JWT HS256 (15min) + Refresh Token (30d) + CSRF Token + HttpOnly Cookies<br/>
    <b>Pagamentos:</b> Multi-gateway (Pagar.me, Asaas, AbacatePay, Mercado Pago) com Router + Circuit Breaker<br/>
    <b>Isolamento de dados:</b> Filtro manual por establishment_id/user_id em queries GORM (sem RLS no banco)<br/>
    """, style_body))

    story.append(Spacer(1, 2*cm))
    story.append(Paragraph(
        f"Relatório gerado automaticamente em {datetime.now().strftime('%d/%m/%Y %H:%M')} — FuuDelivery Security Audit v1.0",
        style_footer
    ))

    # ── Build ───────────────────────────────────────────────────────
    doc.build(story)
    print(f"✅ PDF gerado: {output_path}")

if __name__ == "__main__":
    output = "docs/security-audit/relatorio-auditoria-seguranca.pdf"
    os.makedirs(os.path.dirname(output), exist_ok=True)
    build_pdf(output)
