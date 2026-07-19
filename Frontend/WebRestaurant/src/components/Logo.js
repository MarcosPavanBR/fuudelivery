import React from "react";

const Logo = ({ size = 40, variant = "full", className = "" }) => {
  const MarkIcon = () => (
    <svg
      width={size}
      height={size}
      viewBox="0 0 48 48"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <defs>
        <linearGradient id="fuuGrad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#EA1D2C" />
          <stop offset="50%" stopColor="#FF4444" />
          <stop offset="100%" stopColor="#F7A11E" />
        </linearGradient>
        <linearGradient id="fuuGradDark" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#C41420" />
          <stop offset="100%" stopColor="#EA1D2C" />
        </linearGradient>
        <filter id="shadow" x="-10%" y="-10%" width="120%" height="120%">
          <feDropShadow dx="0" dy="2" stdDeviation="3" floodColor="#EA1D2C" floodOpacity="0.3"/>
        </filter>
      </defs>
      <rect width="48" height="48" rx="14" fill="url(#fuuGrad)" filter="url(#shadow)" />
      <path
        d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z"
        fill="white"
      />
      <circle cx="38" cy="12" r="4" fill="#F7A11E" />
      <path
        d="M35 10l3 2 3-2"
        stroke="white"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        opacity="0.9"
      />
    </svg>
  );

  const FullLogo = () => (
    <div className={`flex items-center gap-2 ${className}`}>
      <MarkIcon />
      <div className="flex flex-col leading-none">
        <span
          style={{
            fontSize: size * 0.55,
            fontWeight: 900,
            color: "#EA1D2C",
            letterSpacing: "-0.5px",
            lineHeight: 1,
          }}
        >
          Fuu
        </span>
        <span
          style={{
            fontSize: size * 0.32,
            fontWeight: 700,
            color: "#1A1A1A",
            letterSpacing: "2px",
            textTransform: "uppercase",
            lineHeight: 1,
            marginTop: 2,
          }}
        >
          Delivery
        </span>
      </div>
    </div>
  );

  const WhiteLogo = () => (
    <div className={`flex items-center gap-2 ${className}`}>
      <svg
        width={size}
        height={size}
        viewBox="0 0 48 48"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <rect width="48" height="48" rx="14" fill="white" fillOpacity="0.15" />
        <path
          d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z"
          fill="white"
        />
        <circle cx="38" cy="12" r="4" fill="#F7A11E" />
      </svg>
      <div className="flex flex-col leading-none">
        <span
          style={{
            fontSize: size * 0.55,
            fontWeight: 900,
            color: "#FFFFFF",
            letterSpacing: "-0.5px",
            lineHeight: 1,
          }}
        >
          Fuu
        </span>
        <span
          style={{
            fontSize: size * 0.32,
            fontWeight: 700,
            color: "rgba(255,255,255,0.85)",
            letterSpacing: "2px",
            textTransform: "uppercase",
            lineHeight: 1,
            marginTop: 2,
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
        width={size * 1.8}
        height={size * 1.8}
        viewBox="0 0 48 48"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          <linearGradient id="loginGrad" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#EA1D2C" />
            <stop offset="50%" stopColor="#FF4444" />
            <stop offset="100%" stopColor="#F7A11E" />
          </linearGradient>
          <filter id="loginShadow" x="-20%" y="-20%" width="140%" height="140%">
            <feDropShadow dx="0" dy="6" stdDeviation="10" floodColor="#EA1D2C" floodOpacity="0.35"/>
          </filter>
        </defs>
        <rect width="48" height="48" rx="14" fill="url(#loginGrad)" filter="url(#loginShadow)" />
        <path
          d="M14 14h12c4.4 0 8 3.6 8 8s-3.6 8-8 8h-4v8h-8V14zm8 12h4c2.2 0 4-1.8 4-4s-1.8-4-4-4h-4v8z"
          fill="white"
        />
        <circle cx="38" cy="12" r="4" fill="#F7A11E" />
        <path
          d="M35 10l3 2 3-2"
          stroke="white"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
          opacity="0.9"
        />
      </svg>
      <div className="mt-4 text-center">
        <h1
          style={{
            fontSize: size * 0.8,
            fontWeight: 900,
            background: "linear-gradient(135deg, #EA1D2C, #FF4444, #F7A11E)",
            WebkitBackgroundClip: "text",
            WebkitTextFillColor: "transparent",
            letterSpacing: "-1px",
          }}
        >
          FuuDelivery
        </h1>
        <p
          style={{
            fontSize: size * 0.28,
            color: "#6B7280",
            fontWeight: 500,
            letterSpacing: "3px",
            textTransform: "uppercase",
            marginTop: 4,
          }}
        >
          Painel do Restaurante
        </p>
      </div>
    </div>
  );

  if (variant === "mark") return <MarkIcon />;
  if (variant === "white") return <WhiteLogo />;
  if (variant === "login") return <LoginLogo />;
  return <FullLogo />;
};

export default Logo;
