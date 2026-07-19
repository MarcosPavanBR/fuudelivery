# Identidade Visual FUUDELIVERY

## Marca

**FUUDELIVERY** — junção de "FUUD" (FOOD com grafia estilizada) + "DELIVERY".

### Logotipos

| Arquivo | Uso |
|---------|-----|
| `logos/logo-horizontal.svg` | Versão principal — fundo claro |
| `logos/logo-horizontal-white.svg` | Versão para fundo escuro |
| `logos/logo-icon.svg` | Ícone do app (512x512) |

### Cores

| Cor | Hex | Uso |
|-----|-----|-----|
| Vermelho principal | `#DC2626` | Ações, botões primários, marca |
| Vermelho escuro | `#B91C1C` | Hover/states |
| Amarelo | `#FBBF24` | Destaques, badges, estrelas |
| Cinza 800 | `#1F2937` | Títulos |
| Cinza 500 | `#6B7280` | Texto secundário |

### Tipografia

- **Display:** Segoe UI / Helvetica Neue (sans-serif)
- **Pesos:** 700 (bold) para títulos, 400 (regular) para corpo, 900 (black) para marca
- **Tamanhos:** 14px corpo, 20px subtítulo, 30px+ para telas

## Aplicação nos Apps

### AppComida / AppEntrega (React Native)

Substitua `constants/Colors.ts` pelo arquivo `colors.ts` na raiz da brand.

### WebRestaurante (React)

Importe `tokens.ts` e use as variáveis CSS via design tokens.

## Boas Práticas

- **Não** distorcer o logotipo — sempre manter proporção
- **Não** usar o ícone sem o texto na versão horizontal
- **Não** aplicar sombras no logotipo
- **Sim** usar a versão white em fundos escuros ou vermelhos
- **Sim** manter área de respiro de 20% nas bordas
