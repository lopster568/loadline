# Deploying the site

The site deploys manually to Netlify (`loadline-dev.netlify.app`); there is
no CI. From `site/`:

    npm run build && npx netlify-cli deploy --prod --dir=dist --site f3a91b08-3d55-4c06-bcb4-7bc6fdc91981

The site id is not a credential; deploys need a Netlify login on the account
that owns the site. A clone that has run `netlify link` once can drop the
`--site` flag.

Verify with a cache-busted fetch, never a browser reload:

    curl -s "https://loadline-dev.netlify.app/tier2.json?cb=$RANDOM" | head -c 200

and check the `schema` field matches `data/tier2-published.json` in the
commit you deployed. `site/public/*.json` are copies of the published data
files; a data change that skips the copy step deploys a site that contradicts
the repo, which is the drift this file exists to prevent.

Optional upgrade, one dashboard click, never done: connect the Netlify site
to the GitHub repo so pushes to main deploy on their own. Until then every
push to `site/` or `data/tier2-published.json` needs the command above.
