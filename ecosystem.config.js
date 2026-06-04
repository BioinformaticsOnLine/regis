module.exports = {
  apps: [
    {
      name: "regis-api",
      script: "./start.sh",
      interpreter: "/bin/bash",
      cwd: "/home/pranjal.p/regis_dev/regis",

      // Keep alive
      autorestart: true,
      watch: false,
      max_restarts: 10,
      restart_delay: 3000,

      // Logging
      out_file: "/home/pranjal.p/regis_dev/regis/logs/pm2-out.log",
      error_file: "/home/pranjal.p/regis_dev/regis/logs/pm2-error.log",
      merge_logs: true,
      log_date_format: "YYYY-MM-DD HH:mm:ss Z",

      // Environment
      env: {
        NODE_ENV: "production",
      },
    },
  ],
};
