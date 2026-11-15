// Site URLs that honour the base path (GitHub Pages serves a project under /<repo>/).
const base = import.meta.env.BASE_URL.replace(/\/$/, '');

export const url = (path = ''): string => `${base}/${path.replace(/^\//, '')}`;

export const REPO = 'https://github.com/afri-bit/truss';
