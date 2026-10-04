if (!process.env.NEXTAUTH_URL) {
    process.env.NEXTAUTH_URL = "https://caresia.vercel.app";
}
if (!process.env.NEXTAUTH_SECRET) {
    process.env.NEXTAUTH_SECRET = "caresia-build-placeholder";
}

/** @type {import('next').NextConfig} */
const nextConfig = {
    experimental: {
        serverActions: true,
    }
}

module.exports = nextConfig
