import React from "react";

const Logo = ({ size = 40, variant = "full", className = "" }) => {
  const MarkIcon = ({ s }) => (
    <svg
      width={s}
      height={s}
      viewBox="0 0 120 120"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <linearGradient id={`fuuGrad-${s}`} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#EA1D2C" />
          <stop offset="100%" stopColor="#C41420" />
        </linearGradient>
        <linearGradient id={`accent-${s}`} x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#FF6B35" />
          <stop offset="100%" stopColor="#F7A11E" />
        </linearGradient>
        <filter id={`shadow-${s}`} x="-10%" y="-5%" width="120%" height="120%">
          <feDropShadow dx="0" dy="4" stdDeviation="6" floodColor="#EA1D2C" floodOpacity="0.25" />
        </filter>
      </defs>
      <rect
        width="120"
        height="120"
        rx="28"
        fill={`url(#fuuGrad-${s})`}
        filter={`url(#shadow-${s})`}
      />
      {/* Stylized F */}
      <path
        d="M32 30h40c6 0 10 4 10 10s-4 10-10 10H42v8h-10V30zm10 16h30c2 0 4-2 4-4s-2-4-4-4H42v8z"
        fill="white"
      />
      {/* Speed lines */}
      <path
        d="M28 68h8M28 76h14M28 84h6"
        stroke="white"
        strokeWidth="3"
        strokeLinecap="round"
        opacity="0.45"
      />
      {/* Accent dot */}
      <circle cx="98" cy="26" r="10" fill={`url(#accent-${s})`} />
      <path
        d="M94 22l4 4 4-4"
        stroke="white"
        strokeWidth="2.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.9"
      />
    </svg>
  );

  const FullLogo = () => (
    <div className={`flex items-center gap-3 ${className}`}>
      <MarkIcon s={size} />
      <div className="flex flex-col leading-none">
        <span
          style={{
            fontSize: size * 0.52,
            fontWeight: 900,
            color: "#EA1D2C",
            letterSpacing: "-0.5px",
            lineHeight: 1,
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          Fuu
        </span>
        <span
          style={{
            fontSize: size * 0.24,
            fontWeight: 700,
            color: "#1A1A1A",
            letterSpacing: "3px",
            textTransform: "uppercase",
            lineHeight: 1,
            marginTop: 3,
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          Delivery
        </span>
      </div>
    </div>
  );

  const WhiteLogo = () => (
    <div className={`flex items-center gap-3 ${className}`}>
      <svg
        width={size}
        height={size}
        viewBox="0 0 120 120"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <rect width="120" height="120" rx="28" fill="white" fillOpacity="0.12" />
        <path
          d="M32 30h40c6 0 10 4 10 10s-4 10-10 10H42v8h-10V30zm10 16h30c2 0 4-2 4-4s-2-4-4-4H42v8z"
          fill="white"
        />
        <path
          d="M28 68h8M28 76h14M28 84h6"
          stroke="white"
          strokeWidth="3"
          strokeLinecap="round"
          opacity="0.35"
        />
        <circle cx="98" cy="26" r="10" fill="#F7A11E" />
        <path
          d="M94 22l4 4 4-4"
          stroke="white"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity="0.9"
        />
      </svg>
      <div className="flex flex-col leading-none">
        <span
          style={{
            fontSize: size * 0.52,
            fontWeight: 900,
            color: "#FFFFFF",
            letterSpacing: "-0.5px",
            lineHeight: 1,
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          Fuu
        </span>
        <span
          style={{
            fontSize: size * 0.24,
            fontWeight: 700,
            color: "rgba(255,255,255,0.8)",
            letterSpacing: "3px",
            textTransform: "uppercase",
            lineHeight: 1,
            marginTop: 3,
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          Delivery
        </span>
      </div>
    </div>
  );

  const LoginLogo = () => (
    <div className={`flex flex-col items-center ${className}`}>
      <svg
        width={size * 2}
        height={size * 2}
        viewBox="0 0 120 120"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <linearGradient id="loginGrad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#EA1D2C" />
            <stop offset="100%" stopColor="#C41420" />
          </linearGradient>
          <linearGradient id="loginAccent" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#FF6B35" />
            <stop offset="100%" stopColor="#F7A11E" />
          </linearGradient>
          <filter id="loginShadow" x="-15%" y="-10%" width="130%" height="130%">
            <feDropShadow dx="0" dy="8" stdDeviation="12" floodColor="#EA1D2C" floodOpacity="0.3" />
          </filter>
        </defs>
        <rect
          width="120"
          height="120"
          rx="28"
          fill="url(#loginGrad)"
          filter="url(#loginShadow)"
        />
        <path
          d="M32 30h40c6 0 10 4 10 10s-4 10-10 10H42v8h-10V30zm10 16h30c2 0 4-2 4-4s-2-4-4-4H42v8z"
          fill="white"
        />
        <path
          d="M28 68h8M28 76h14M28 84h6"
          stroke="white"
          strokeWidth="3"
          strokeLinecap="round"
          opacity="0.45"
        />
        <circle cx="98" cy="26" r="10" fill="url(#loginAccent)" />
        <path
          d="M94 22l4 4 4-4"
          stroke="white"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity="0.9"
        />
      </svg>
      <div className="mt-5 text-center">
        <h1
          style={{
            fontSize: size * 0.85,
            fontWeight: 900,
            background: "linear-gradient(135deg, #EA1D2C, #C41420)",
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
            letterSpacing: "-1px",
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          FuuDelivery
        </h1>
        <p
          style={{
            fontSize: size * 0.26,
            color: "#6B7280",
            fontWeight: 500,
            letterSpacing: "4px",
            textTransform: "uppercase",
            marginTop: 6,
            fontFamily: "Inter, system-ui, sans-serif",
          }}
        >
          Painel do Restaurante
        </p>
      </div>
    </div>
  );

  const MarkOnly = () => <MarkIcon s={size} />;

  if (variant === "mark") return <MarkOnly />;
  if (variant === "white") return <WhiteLogo />;
  if (variant === "login") return <LoginLogo />;
  return <FullLogo />;
};

export default Logo;
