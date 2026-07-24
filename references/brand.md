# FuuDelivery — Identidade Visual

## Marca

### Logomarca (Icone)
O icone da FuuDelivery e um "F" estilizado dentro de um quadrado arredondado com gradiente vermelho. Linhas de velocidade transmitem agilidade na entrega. Um ponto laranja/dourado no canto superior direito representa notificacao e frescor.

### Logotipo (Texto)
- **Fuu**: vermelho (#EA1D2C), peso 900, fonte Inter
- **Delivery**: escuro (#1A1A1A), peso 700, letterSpacing 3px, uppercase

### Logomarca Completa (Icone + Texto)
Combinacao do icone com o logotipo, alinhados verticalmente.

## Cores

| Cor | Hex | Uso |
|---|---|---|
| Fuu Red | #EA1D2C | Cor primaria, botoes, titulos |
| Fuu Dark Red | #C41420 | Gradiente, hover states |
| Fuu Orange | #FF6B35 | Acentos, alertas |
| Fuu Yellow | #F7A11E | Acento do ponto, sucesso |
| Dark | #1A1A1A | Texto principal |
| Gray | #6B7280 | Texto secundario |
| Light Gray | #F3F4F6 | Fundo, bordas |

### Gradiente da Marca
```
linear-gradient(135deg, #EA1D2C, #C41420)
```

### Gradiente do Acento
```
linear-gradient(135deg, #FF6B35, #F7A11E)
```

## Tipografia

- **Fonte principal**: Inter (Google Fonts)
- **Pesos**: 300 (light), 400 (regular), 500 (medium), 600 (semibold), 700 (bold), 800 (extrabold), 900 (black)
- **Fallback**: system-ui, sans-serif

## Tamanhos do Logo

| Variante | Tamanho | Uso |
|---|---|---|
| Mark (icone) | 32-48px | Sidebar, favicon |
| Full (icone + texto) | 40-60px | Header, nav |
| Login | 70-140px | Tela de login |
| White | 32-48px | Fundos escuros |

## Variantes

| Variant | Descricao | Fundo |
|---|---|---|
| `mark` | Apenas o icone | Qualquer |
| `full` | Icone + FuuDelivery | Claro |
| `white` | Versao branca | Escuro |
| `login` | Grande com tulo | Split screen |

## Arquivos

| Arquivo | Descricao |
|---|---|
| `public/brand/logo-mark.svg` | Logomarca (icone) |
| `public/brand/logo-full.svg` | Logomarca completa |
| `public/brand/logo-logotype.svg` | Apenas o texto |
| `public/brand/logo-mark-white.svg` | Versao branca |
| `public/favicon.svg` | Favicon 32x32 |

## Uso

```jsx
import Logo from "./components/Logo";

// Icone apenas (sidebar)
<Logo size={32} variant="mark" />

// Logo completo (header)
<Logo size={40} variant="full" />

// Fundo escuro (sidebar escura)
<Logo size={36} variant="white" />

// Tela de login
<Logo size={70} variant="login" />
```

## Regras

1. **Nao distorcer** o logo — sempre manter proporcao
2. **Espaco livre** — manter ao menos 1x o tamanho do icone ao redor
3. **Fundo claro** — usar variante `full` ou `mark`
4. **Fundo escuro** — usar variante `white`
5. **Nao alterar cores** — usar apenas as paletas documentadas
6. **Tamanho minimo** — icone: 24px, texto: 12px
